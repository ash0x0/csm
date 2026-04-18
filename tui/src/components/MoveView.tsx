import React, { useState, useEffect } from 'react';
import { Box, Text, useInput } from 'ink';
import { TextInput, Spinner } from '@inkjs/ui';
import { listProjects } from '../csm.js';
import type { SessionMeta } from '../types.js';

interface Props {
  session: SessionMeta;
  onMove: (dest: string) => void;
  onCancel: () => void;
}

export function MoveView({ session, onMove, onCancel }: Props) {
  const [projects, setProjects] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [filter, setFilter] = useState('');
  const [cursor, setCursor] = useState(0);
  const [isTyping, setIsTyping] = useState(false);

  useEffect(() => {
    listProjects()
      .then(data => { setProjects(data.filter(p => p !== session.project)); setLoading(false); })
      .catch(() => setLoading(false));
  }, [session.project]);

  const filtered = filter
    ? projects.filter(p => p.toLowerCase().includes(filter.toLowerCase()))
    : projects;

  useInput((input, key) => {
    if (isTyping) return;
    if (key.escape) { onCancel(); return; }
    if (key.upArrow) { setCursor(c => Math.max(0, c - 1)); return; }
    if (key.downArrow) { setCursor(c => Math.min(filtered.length - 1, c + 1)); return; }
    if (key.return && filtered[cursor]) { onMove(filtered[cursor]); return; }
    if (input === '/') { setIsTyping(true); return; }
  }, { isActive: !isTyping });

  return (
    <Box flexDirection="column" padding={1}>
      <Box marginBottom={1}>
        <Text bold>Move: </Text>
        <Text color="cyan">{session.title}</Text>
        <Text dimColor>  ↑↓ nav  enter select  / filter  esc cancel</Text>
      </Box>
      <Text dimColor>Current: <Text color="yellow">{session.project}</Text></Text>

      <Box marginTop={1} marginBottom={1} borderStyle="single" paddingX={1}>
        <Text dimColor>/ </Text>
        {isTyping ? (
          <TextInput
            placeholder="Filter projects or type a path..."
            onChange={v => { setFilter(v); setCursor(0); }}
            onSubmit={v => { setIsTyping(false); if (v) setFilter(v); }}
          />
        ) : (
          <Text>{filter || <Text dimColor>Filter projects... (press /)</Text>}</Text>
        )}
      </Box>

      {loading && <Spinner label="Loading projects..." />}
      {!loading && filtered.length === 0 && (
        <Text dimColor>No matching projects. Press / and type a full path.</Text>
      )}
      {filtered.map((p, i) => (
        <Box key={p}>
          <Text color={i === cursor ? 'green' : undefined} bold={i === cursor}>
            {i === cursor ? '> ' : '  '}
            <Text>{(p.split('/').pop() ?? p).padEnd(25)}</Text>
            <Text dimColor>  {p}</Text>
          </Text>
        </Box>
      ))}
    </Box>
  );
}
