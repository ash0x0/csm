export interface SessionMeta {
  id: string;
  short_id: string;
  title: string;
  project: string;
  branch: string;
  messages: number;
  created: string;
  modified: string;
  file_size: number;
  file_path: string;
  is_active: boolean;
  slug?: string;
}

export interface TimelineEvent {
  time: string;
  type: 'user' | 'assistant' | 'turn-duration' | 'compact' | 'queue';
  summary: string;
  duration_ms?: number;
  tokens_out?: number;
  pre_tokens?: number;
  trigger?: string;
}

export interface Task {
  id: string;
  subject: string;
  description: string;
  activeForm: string;
  status: 'pending' | 'in_progress' | 'completed';
  blocks: string[];
  blockedBy: string[];
}

export interface PlanEntry {
  slug: string;
  session_id: string;
  title: string;
  project: string;
  modified: string;
  path: string;
}

export interface ActivityStats {
  totalSessions: number;
  totalMessages: number;
  firstSessionDate: string;
  dailyActivity: Array<{
    date: string;
    messageCount: number;
    sessionCount: number;
    toolCallCount: number;
  }>;
  hourCounts: Record<string, number>;
}

export interface DiffResult {
  relationship: 'identical' | 'a-contains-b' | 'b-contains-a' | 'diverged' | 'unrelated';
  commonCount: number;
  onlyACount: number;
  onlyBCount: number;
}

export interface SearchResult {
  session: SessionMeta;
  hits: Array<{ context: string; type: 'title' | 'last-prompt' | 'user-prompt' }>;
}
