#!/usr/bin/env node
/**
 * workflow-agent.ts
 * Executa steps `run:` de jobs do GitHub Actions localmente via Docker.
 * Saída: JSON estruturado linha a linha para consumo pelo agente OpenCode.
 *
 * Uso:
 *   npx tsx scripts/workflow-agent.ts <workflow.yml> [job-id] [--dry-run]
 *
 * Exemplos:
 *   npx tsx scripts/workflow-agent.ts .github/workflows/ci.yml
 *   npx tsx scripts/workflow-agent.ts .github/workflows/ci.yml go-test
 *   npx tsx scripts/workflow-agent.ts .github/workflows/ci.yml go-test --dry-run
 */
import fs from 'node:fs';
import path from 'node:path';
import os from 'node:os';
import { spawn, execSync } from 'node:child_process';
import YAML from 'yaml';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
type Step = {
  name?: string;
  run?: string;
  env?: Record<string, string>;
  'working-directory'?: string;
  uses?: string;
  'continue-on-error'?: boolean;
};

type Job = {
  name?: string;
  env?: Record<string, string>;
  steps?: Step[];
  container?: string | { image?: string };
  defaults?: { run?: { shell?: string; 'working-directory'?: string } };
  needs?: string | string[];
};

type Workflow = { jobs?: Record<string, Job> };

// ---------------------------------------------------------------------------
// Jobs que requerem secrets/serviços externos — pular localmente
// ---------------------------------------------------------------------------
const SKIP_JOBS = new Set([
  'secrets-scan',
  'semgrep',
  'sonarcloud',
  'codeql',
  'snyk',
  'dependabot',
]);

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------
function emit(event: Record<string, unknown>): void {
  process.stdout.write(JSON.stringify({ ts: new Date().toISOString(), ...event }) + os.EOL);
}

function mergeEnv(...parts: Array<Record<string, string> | undefined>): Record<string, string> {
  return Object.assign({}, ...parts.filter(Boolean)) as Record<string, string>;
}

function resolveContainer(job: Job): string {
  if (!job.container) return 'ubuntu:22.04';
  if (typeof job.container === 'string') return job.container;
  return job.container.image ?? 'ubuntu:22.04';
}

function readWorkflow(file: string): Workflow {
  const raw = fs.readFileSync(path.resolve(file), 'utf8');
  return YAML.parse(raw) as Workflow;
}

// ---------------------------------------------------------------------------
// Host tool detection — bind-mount Go toolchain into containers
// ---------------------------------------------------------------------------
function getGoMountArgs(): string[] {
  try {
    const goroot = execSync('go env GOROOT', { encoding: 'utf8', timeout: 5000 }).trim();
    const gopath = execSync('go env GOPATH', { encoding: 'utf8', timeout: 5000 }).trim();
    const gobin = path.join(goroot, 'bin/go');
    fs.accessSync(gobin);
    const modCache = path.join(gopath, 'pkg/mod');
    const mounts: string[] = [
      '-v', `${goroot}:/goroot:ro`,
      '-e', `GOROOT=/goroot`,
      '-e', `GOPATH=/gopath`,
      '-e', `PATH=/goroot/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin`,
      '-e', `CGO_ENABLED=1`,
    ];
    if (fs.existsSync(modCache)) {
      mounts.push('-v', `${modCache}:/gopath/pkg/mod:ro`);
    }
    return mounts;
  } catch {
    return [];
  }
}

function getGoModMountArgs(): string[] {
  try {
    const gopath = execSync('go env GOPATH', { encoding: 'utf8', timeout: 5000 }).trim();
    const modCache = path.join(gopath, 'pkg/mod');
    const mounts: string[] = ['-e', `GOPATH=/gopath`];
    if (fs.existsSync(modCache)) {
      mounts.push('-v', `${modCache}:/gopath/pkg/mod:ro`);
    }
    return mounts;
  } catch {
    return [];
  }
}

// ---------------------------------------------------------------------------
// Step runner — executes a command inside a shared Docker container
// ---------------------------------------------------------------------------
async function execInContainer(
  containerId: string,
  shell: string,
  command: string,
): Promise<number> {
  const args = [
    'exec',
    '-i',
    containerId,
    shell, '-c', command,
  ];

  return new Promise<number>((resolve, reject) => {
    const child = spawn('docker', args, { stdio: ['ignore', 'pipe', 'pipe'] });
    child.stdout.on('data', (buf) => {
      for (const line of String(buf).split(/\r?\n/)) {
        if (line.trim()) emit({ type: 'stdout', line });
      }
    });
    child.stderr.on('data', (buf) => {
      for (const line of String(buf).split(/\r?\n/)) {
        if (line.trim()) emit({ type: 'stderr', line });
      }
    });
    child.on('close', (code) => resolve(code ?? 1));
    child.on('error', reject);
  });
}

// ---------------------------------------------------------------------------
// Job runner — uses a single persistent container for all steps
// ---------------------------------------------------------------------------
async function runJob(
  jobId: string,
  job: Job,
  projectRoot: string,
  dryRun: boolean,
): Promise<number> {
  if (SKIP_JOBS.has(jobId)) {
    emit({ type: 'job_skipped', job: jobId, reason: 'requires_external_secrets_or_services' });
    return 0;
  }

  const image = resolveContainer(job);
  const shell = job.defaults?.run?.shell ?? 'bash';
  const jobDefaultWd = path.resolve(projectRoot, job.defaults?.run?.['working-directory'] ?? '.');
  const jobEnv = mergeEnv(job.env);

  emit({ type: 'job_started', job: jobId, name: job.name ?? jobId, image, shell });

  const steps = (job.steps ?? []).filter((s) => s.run);

  if (dryRun) {
    for (const [i, step] of steps.entries()) {
      emit({
        type: 'step_dry_run',
        job: jobId,
        step: step.name ?? `step_${i + 1}`,
        command: step.run,
        workingDir: step['working-directory'] ?? job.defaults?.run?.['working-directory'] ?? '.',
      });
    }
    emit({ type: 'job_finished', job: jobId, status: 'dry_run' });
    return 0;
  }

  const imageHasGo = image.includes('golang');
  const goMounts = imageHasGo ? getGoModMountArgs() : getGoMountArgs();
  const hasGoStep = steps.some((s) => s.run?.includes('go '));

  // Generate a unique container name
  const containerTimestamp = Date.now();
  const containerName = `wfa-${jobId}-${containerTimestamp}`;

  // Clean up any leftover containers from previous runs
  try { execSync(`docker rm -f ${containerName}`, { stdio: 'ignore', timeout: 5000 }); } catch { /* best effort */ }

  // Start a persistent container (sleeps forever so we can exec into it)
  const runArgs = [
    'run', '-d',
    '--name', containerName,
    '-v', `${projectRoot}:/workspace`,
    '-w', '/workspace',
    ...(hasGoStep ? goMounts : []),
    image,
    'sleep', 'infinity',
  ];

  // Install missing tools in the shared container
  const missingTools: string[] = [];
  if (hasGoStep) {
    const gitCheckCode = await execInContainer(containerName, shell, 'command -v git');
    if (gitCheckCode !== 0) missingTools.push('git');
  }

  if (missingTools.length > 0) {
    const installCmd = `apt-get update -qq && apt-get install -y -qq ${missingTools.join(' ')} 2>&1`;
    emit({ type: 'step_started', job: jobId, step: `Install missing tools: ${missingTools.join(', ')}` });
    const code = await execInContainer(containerName, shell, installCmd);
    emit({ type: 'step_finished', job: jobId, step: `Install missing tools: ${missingTools.join(', ')}`, exitCode: code });
    if (code !== 0) {
      emit({ type: 'job_finished', job: jobId, status: 'failed', failedStep: `Install missing tools: ${missingTools.join(', ')}` });
      return 1;
    }
  }
  }

  let jobFailed = false;
  let failedStepName: string | undefined;

  for (const [i, step] of steps.entries()) {
    const stepName = step.name ?? `step_${i + 1}`;
    const stepEnv = mergeEnv(jobEnv, step.env);
    const stepWd = step['working-directory']
      ? path.resolve(projectRoot, step['working-directory'])
      : jobDefaultWd;

    // Build the command: change to working directory, set env vars, run command
    const relDir = path.relative(projectRoot, stepWd) || '.';
    const envExport = Object.entries(stepEnv)
      .map(([k, v]) => `export ${k}=${JSON.stringify(v)}`)
      .join('; ');
    const fullCommand = envExport
      ? `${envExport}; cd ${JSON.stringify(relDir)} && ${step.run!}`
      : `cd ${JSON.stringify(relDir)} && ${step.run!}`;

    emit({ type: 'step_started', job: jobId, step: stepName });

    const code = await execInContainer(containerName, shell, fullCommand);

    emit({ type: 'step_finished', job: jobId, step: stepName, exitCode: code });

    if (code !== 0 && !step['continue-on-error']) {
      jobFailed = true;
      failedStepName = stepName;
      break;
    }
  }

  // Clean up the container
  try {
    await new Promise<void>((resolve, reject) => {
      const child = spawn('docker', ['rm', '-f', containerName], { stdio: 'ignore' });
      child.on('close', (code) => {
        if (code !== 0) emit({ type: 'debug', message: `Failed to remove container ${containerName}` });
        resolve();
      });
      child.on('error', () => resolve());
    });
  } catch {
    // best effort cleanup
  }

  if (jobFailed) {
    emit({ type: 'job_finished', job: jobId, status: 'failed', failedStep: failedStepName });
    return 1;
  }

  emit({ type: 'job_finished', job: jobId, status: 'success' });
  return 0;
}

// ---------------------------------------------------------------------------
// Main
// ---------------------------------------------------------------------------
async function main(): Promise<void> {
  const args = process.argv.slice(2);
  const dryRun = args.includes('--dry-run');
  const filtered = args.filter((a) => a !== '--dry-run');

  const workflowPath = filtered[0];
  const targetJob = filtered[1];

  if (!workflowPath) {
    emit({ type: 'error', message: 'Uso: npx tsx scripts/workflow-agent.ts <workflow.yml> [job-id] [--dry-run]' });
    process.exit(1);
  }

  const workflow = readWorkflow(workflowPath);
  const projectRoot = process.cwd();
  const allJobs = workflow.jobs ?? {};

  let selected: Array<[string, Job]>;
  if (targetJob) {
    if (!allJobs[targetJob]) {
      emit({ type: 'error', message: `Job não encontrado: ${targetJob}`, available: Object.keys(allJobs) });
      process.exit(1);
    }
    selected = [[targetJob, allJobs[targetJob]]];
  } else {
    selected = Object.entries(allJobs) as Array<[string, Job]>;
  }

  let failures = 0;
  for (const [jobId, job] of selected) {
    const code = await runJob(jobId, job, projectRoot, dryRun);
    if (code !== 0) failures += 1;
  }

  emit({
    type: 'workflow_finished',
    status: failures === 0 ? 'success' : 'failed',
    failures,
  });

  process.exit(failures === 0 ? 0 : 1);
}

main().catch((err: unknown) => {
  emit({ type: 'error', message: err instanceof Error ? err.message : String(err) });
  process.exit(1);
});
