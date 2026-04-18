import React, { useState, useEffect } from 'react';
import { Box, Text, useInput } from 'ink';
import { Spinner } from '@inkjs/ui';
import { diffSessions } from '../csm.js';
import type { SessionMeta, DiffResult } from '../types.js';

interface Props {
  sessionA: SessionMeta;
  sessionB: SessionMeta;
  onBack: () => void;
}

const RELATIONSHIP_LABEL: Record<string, string> = {
  identical:      'Identical — both sessions share the same history',
  'a-contains-b': 'A is a superset of B (A extends B)',
  'b-contains-a': 'B is a superset of A (B extends A)',
  diverged:       'Diverged — shared history then split',
  unrelated:      'Unrelated — no shared history',
};

const RELATIONSHIP_COLOR: Record<string, string> = {
  identical:      'green',
  'a-contains-b': 'cyan',
  'b-contains-a': 'cyan',
  diverged:       'yellow',
  unrelated:      'red',
};

export function DiffView({ sessionA, sessionB, onBack }: Props) {
  const [result, setResult] = useState<DiffResult | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    diffSessions(sessionA.short_id, sessionB.short_id)
      .then(data => { setResult(data); setLoading(false); })
      .catch(e => { setError(String(e)); setLoading(false); });
  }, [sessionA.short_id, sessionB.short_id]);

  useInput((input, key) => {
    if (key.escape || input === 'q') onBack();
  });

  return (
    <Box flexDirection="column" padding={1}>
      <Box marginBottom={1}>
        <Text bold>Diff</Text>
        <Text dimColor>  (esc to go back)</Text>
      </Box>

      <Box flexDirection="column" marginBottom={1} gap={1}>
        <Box gap={2}>
          <Text color="cyan" bold>A</Text>
          <Text>{sessionA.short_id}</Text>
          <Text dimColor>{sessionA.title.length > 50 ? sessionA.title.slice(0, 47) + '...' : sessionA.title}</Text>
        </Box>
        <Box gap={2}>
          <Text color="yellow" bold>B</Text>
          <Text>{sessionB.short_id}</Text>
          <Text dimColor>{sessionB.title.length > 50 ? sessionB.title.slice(0, 47) + '...' : sessionB.title}</Text>
        </Box>
      </Box>

      {loading && <Spinner label="Comparing sessions..." />}
      {error && <Text color="red">Error: {error}</Text>}

      {result && (
        <Box flexDirection="column" marginTop={1} gap={1}>
          <Box borderStyle="round" paddingX={2} paddingY={1}>
            <Text color={RELATIONSHIP_COLOR[result.relationship] ?? 'white'} bold>
              {RELATIONSHIP_LABEL[result.relationship] ?? result.relationship}
            </Text>
          </Box>
          <Box flexDirection="column" marginTop={1}>
            {result.commonCount > 0 && (
              <Text>  Shared events:  <Text color="green">{result.commonCount}</Text></Text>
            )}
            {result.onlyACount > 0 && (
              <Text>  Only in A:      <Text color="cyan">{result.onlyACount}</Text></Text>
            )}
            {result.onlyBCount > 0 && (
              <Text>  Only in B:      <Text color="yellow">{result.onlyBCount}</Text></Text>
            )}
          </Box>
          {result.relationship === 'diverged' && (
            <Box marginTop={1}>
              <Text dimColor>Tip: press esc, keep both selected, then enter to merge.</Text>
            </Box>
          )}
        </Box>
      )}
    </Box>
  );
}
