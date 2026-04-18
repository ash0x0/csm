import React from 'react';
import { Box, Text } from 'ink';
import type { SessionMeta } from '../types.js';

interface Props {
  sessions: SessionMeta[];
  selectedIndex: number;
  selectedIds: Set<string>;
  onIndexChange: (index: number) => void;
  onToggleSelect: (id: string) => void;
  height: number;
}

function groupByProject(sessions: SessionMeta[]): Map<string, SessionMeta[]> {
  const groups = new Map<string, SessionMeta[]>();
  for (const s of sessions) {
    const projectName = s.project.split('/').pop() ?? s.project;
    if (!groups.has(projectName)) groups.set(projectName, []);
    groups.get(projectName)!.push(s);
  }
  return groups;
}

function relativeTime(iso: string): string {
  const diff = Date.now() - new Date(iso).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 60) return `${mins}m`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h`;
  return `${Math.floor(hrs / 24)}d`;
}

export function SessionList({ sessions, selectedIndex, selectedIds, height }: Props) {
  type Row =
    | { type: 'header'; project: string }
    | { type: 'session'; session: SessionMeta; flatIndex: number };

  const rows: Row[] = [];
  let flatIndex = 0;
  const groups = groupByProject(sessions);

  for (const [project, group] of groups) {
    rows.push({ type: 'header', project });
    for (const s of group) {
      rows.push({ type: 'session', session: s, flatIndex: flatIndex++ });
    }
  }

  const visibleCount = Math.max(1, height - 4);
  const sessionRows = rows.filter((r): r is Extract<Row, { type: 'session' }> => r.type === 'session');
  const totalSessions = sessionRows.length;
  const viewStart = Math.max(0, Math.min(selectedIndex - Math.floor(visibleCount / 2), totalSessions - visibleCount));

  const visibleFlatIndices = new Set<number>();
  for (let i = viewStart; i < Math.min(viewStart + visibleCount, totalSessions); i++) {
    visibleFlatIndices.add(i);
  }

  const visibleRows = rows.filter(r => {
    if (r.type === 'header') {
      const group = groups.get(r.project)!;
      return group.some(s => {
        const fi = sessionRows.findIndex(sr => sr.session === s);
        return visibleFlatIndices.has(fi);
      });
    }
    return visibleFlatIndices.has(r.flatIndex);
  });

  return (
    <Box flexDirection="column" flexGrow={1} overflow="hidden">
      {visibleRows.map((row, i) => {
        if (row.type === 'header') {
          return (
            <Text key={`h-${row.project}-${i}`} dimColor>
              {' ▼ '}{row.project}
            </Text>
          );
        }

        const { session, flatIndex: fi } = row;
        const isCursor = fi === selectedIndex;
        const isMultiSelected = selectedIds.has(session.short_id);
        const title = session.title.length > 42 ? session.title.slice(0, 39) + '...' : session.title;
        const activeMarker = session.is_active ? '*' : ' ';

        return (
          <Box key={session.id}>
            <Text color={isCursor ? 'green' : undefined} bold={isCursor}>
              {isCursor ? '>' : ' '}
              {isMultiSelected ? '[x]' : '   '}
              {' '}
              <Text dimColor>{session.short_id}</Text>
              {activeMarker}
              {'  '}
              {title.padEnd(44)}
              {'  '}
              <Text dimColor>{String(session.messages).padStart(4)} msgs  {relativeTime(session.modified)}</Text>
            </Text>
          </Box>
        );
      })}
    </Box>
  );
}
