import { execa } from 'execa';
import type {
  SessionMeta, TimelineEvent, Task, PlanEntry,
  ActivityStats, DiffResult, SearchResult
} from './types.js';

let csmBin = 'csm';
let claudeDir = '';

export function init(bin: string, dir: string): void {
  csmBin = bin;
  claudeDir = dir;
}

async function run(args: string[]): Promise<string> {
  const fullArgs = claudeDir ? ['--claude-dir', claudeDir, ...args] : args;
  const result = await execa(csmBin, fullArgs, { reject: true });
  return result.stdout;
}

export async function listSessions(): Promise<SessionMeta[]> {
  const out = await run(['list', '--json']);
  return JSON.parse(out) as SessionMeta[];
}

export async function getTimeline(id: string): Promise<TimelineEvent[]> {
  const out = await run(['timeline', id, '--json']);
  return JSON.parse(out) as TimelineEvent[];
}

export async function getTasks(id: string): Promise<Task[]> {
  const out = await run(['tasks', id, '--json']);
  return JSON.parse(out) as Task[];
}

export async function getPlans(): Promise<PlanEntry[]> {
  const out = await run(['plans', '--json']);
  return JSON.parse(out) as PlanEntry[];
}

export async function getActivity(): Promise<ActivityStats> {
  const out = await run(['activity', '--json']);
  return JSON.parse(out) as ActivityStats;
}

export async function searchSessions(query: string): Promise<SearchResult[]> {
  const out = await run(['search', query, '--json']);
  return JSON.parse(out) as SearchResult[];
}

export async function diffSessions(idA: string, idB: string): Promise<DiffResult> {
  const out = await run(['diff', idA, idB, '--json']);
  return JSON.parse(out) as DiffResult;
}

export async function deleteSession(id: string): Promise<void> {
  await run(['rm', '--force', id]);
}

export async function cloneSession(id: string): Promise<string> {
  const out = await run(['clone', id]);
  // Output: "Cloned abc12345 → def67890\nResume with: ..."
  const match = out.match(/→ (\S+)/);
  return match?.[1] ?? '';
}

export interface MergeResult {
  newId: string;
  strategy: string;
  totalEvents: number;
}

export async function mergeSession(ids: string[]): Promise<MergeResult> {
  const out = await run(['merge', ...ids]);
  const idMatch = out.match(/Created merged session: (\S+)/);
  const statsMatch = out.match(/Strategy: (\S+) \| events: (\d+)/);
  return {
    newId: idMatch?.[1] ?? '',
    strategy: statsMatch?.[1] ?? '',
    totalEvents: statsMatch ? parseInt(statsMatch[2], 10) : 0,
  };
}

export interface DryRunResult {
  strategy: string;
  sharedCount: number;
  branchAOnly: number;
  branchBOnly: number;
  totalEvents: number;
  warnings: string[];
}

export async function dryRunMerge(ids: string[]): Promise<DryRunResult> {
  const fullArgs = claudeDir
    ? ['--claude-dir', claudeDir, 'merge', '--dry-run', ...ids]
    : ['merge', '--dry-run', ...ids];
  const result = await execa(csmBin, fullArgs, { reject: true, all: true });
  const stdout = result.stdout ?? '';
  const stderr = result.stderr ?? '';

  const strategy = stdout.match(/^Strategy:\s+(.+)$/m)?.[1]?.trim() ?? '';
  const sharedCount = parseInt(stdout.match(/^Shared events:\s+(\d+)$/m)?.[1] ?? '0', 10);
  const branchAOnly = parseInt(stdout.match(/^Session A unique:\s+(\d+)$/m)?.[1] ?? '0', 10);
  const branchBOnly = parseInt(stdout.match(/^Session B unique:\s+(\d+)$/m)?.[1] ?? '0', 10);
  const totalEvents = parseInt(stdout.match(/^Total output events:\s+(\d+)$/m)?.[1] ?? '0', 10);

  const warnings = stderr
    .split('\n')
    .filter(l => l.includes('compact') || l.toLowerCase().includes('warn'))
    .map(l => l.trim())
    .filter(Boolean);

  return { strategy, sharedCount, branchAOnly, branchBOnly, totalEvents, warnings };
}

export async function listProjects(): Promise<string[]> {
  const sessions = await listSessions();
  return [...new Set(sessions.map(s => s.project))].sort();
}

export async function moveSession(id: string, dest: string): Promise<void> {
  await run(['move', id, dest]);
}
