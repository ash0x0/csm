import React from 'react';
import { Box, Text } from 'ink';
import type { SessionMeta } from '../types.js';

export type Row =
  | { type: 'header'; project: string; displayName: string }
  | { type: 'session'; session: SessionMeta; sessionIndex: number };

interface Props {
  sessions: SessionMeta[];
  collapsedGroups: Set<string>;
  onToggleCollapse: (project: string) => void;
  selectedIndex: number;
  selectedIds: Set<string>;
  onIndexChange: (index: number) => void;
  onToggleSelect: (id: string) => void;
  height: number;
}

function groupByProject(sessions: SessionMeta[]): Map<string, SessionMeta[]> {
  const groups = new Map<string, SessionMeta[]>();
  for (const s of sessions) {
    const key = s.project;
    if (!groups.has(key)) groups.set(key, []);
    groups.get(key)!.push(s);
  }
  return groups;
}

function getDisplayName(project: string, allProjects: string[]): string {
  const lastSegment = project.split('/').pop() ?? project;
  const collisions = allProjects.filter(p => (p.split('/').pop() ?? p) === lastSegment);
  if (collisions.length <= 1) return lastSegment;
  const parts = project.split('/');
  return parts.slice(-2).join('/');
}

export function buildRows(sessions: SessionMeta[], collapsedGroups: Set<string>): Row[] {
  const groups = groupByProject(sessions);
  const allProjects = Array.from(groups.keys());
  const rows: Row[] = [];
  let sessionIndex = 0;

  for (const [project, group] of groups) {
    const displayName = getDisplayName(project, allProjects);
    rows.push({ type: 'header', project, displayName });
    if (!collapsedGroups.has(project)) {
      for (const s of group) {
        rows.push({ type: 'session', session: s, sessionIndex: sessionIndex++ });
      }
    } else {
      sessionIndex += group.length;
    }
  }

  return rows;
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h`;
  return `${Math.floor(hrs / 24)}d`;
}

export function SessionList({ sessions, collapsedGroups, selectedIndex, selectedIds, height }: Props) {
  const rows = buildRows(sessions, collapsedGroups);

  const visibleCount = Math.max(1, height - 2);
  const viewStart = Math.max(0, Math.min(selectedIndex - Math.floor(visibleCount / 2), rows.length - visibleCount));
  const visibleRows = rows.slice(viewStart, viewStart + visibleCount);

  return (
    <Box flexDirection="column" flexGrow={1} overflow="hidden">
      {visibleRows.map((row, i) => {
        const rowIndex = viewStart + i;
        const isCursor = rowIndex === selectedIndex;

        if (row.type === 'header') {
          const isCollapsed = collapsedGroups.has(row.project);
          return (
            <Text key={`h-${row.project}-${i}`} color={isCursor ? 'cyanBright' : 'magenta'} bold>
              {isCursor ? '>' : ' '}
              {' '}
              <Text color="cyan">{isCollapsed ? '▶' : '▼'}</Text>
              {' '}
              {row.displayName}
            </Text>
          );
        }

        const { session } = row;
        const isMultiSelected = selectedIds.has(session.short_id);
        const title = session.title.length > 42 ? session.title.slice(0, 39) + '...' : session.title;
        const activeMarker = session.is_active ? '*' : ' ';

        return (
          <Box key={session.id}>
            <Text color={isCursor ? 'cyanBright' : undefined} bold={isCursor}>
              <Text color={isCursor ? 'greenBright' : undefined}>{isCursor ? '>' : ' '}</Text>
              {isMultiSelected ? <Text color="yellow">[x]</Text> : '   '}
              {' '}
              <Text color="blue">{session.short_id}</Text>
              {activeMarker}
              {'  '}
              {title.padEnd(44)}
              {'  '}
              <Text color="gray">{String(session.messages).padStart(4)} msgs  {relativeTime(session.modified)}</Text>
            </Text>
          </Box>
        );
      })}
    </Box>
  );
}
