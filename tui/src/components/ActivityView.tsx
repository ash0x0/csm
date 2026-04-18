import React, { useState, useEffect } from 'react';
import { Box, Text, useInput } from 'ink';
import { Spinner } from '@inkjs/ui';
import { getActivity } from '../csm.js';
import type { ActivityStats } from '../types.js';

interface Props {
  onBack: () => void;
}

const BLOCKS = [' ', '░', '▒', '▓', '█'];

function toBlock(count: number, max: number): string {
  if (max === 0) return BLOCKS[0];
  return BLOCKS[Math.min(4, Math.ceil((count / max) * 4))];
}

export function ActivityView({ onBack }: Props) {
  const [stats, setStats] = useState<ActivityStats | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getActivity()
      .then(data => { setStats(data); setLoading(false); })
      .catch(() => setLoading(false));
  }, []);

  useInput((input, key) => {
    if (key.escape || input === 'q') onBack();
  });

  if (loading) return <Box padding={1}><Spinner label="Loading activity..." /></Box>;
  if (!stats) return <Box padding={1}><Text color="red">Failed to load activity data.</Text></Box>;

  const last90 = stats.dailyActivity.slice(-90);
  const maxMsg = Math.max(...last90.map(d => d.messageCount), 1);

  const weeks: typeof last90[] = [];
  for (let i = 0; i < last90.length; i += 7) {
    weeks.push(last90.slice(i, i + 7));
  }

  return (
    <Box flexDirection="column" padding={1}>
      <Box marginBottom={1}>
        <Text bold>Activity</Text>
        <Text dimColor>  (esc to go back)</Text>
      </Box>
      <Box gap={2} marginBottom={1}>
        <Text>Total sessions: <Text color="cyan">{stats.totalSessions}</Text></Text>
        <Text>Total messages: <Text color="green">{stats.totalMessages}</Text></Text>
      </Box>
      <Text dimColor bold>Last 90 days (each cell = 1 day, darker = more messages)</Text>
      <Box flexDirection="column" marginTop={1}>
        {weeks.map((week, wi) => (
          <Box key={wi}>
            {week.map((day, di) => (
              <Text key={di} color="green">{toBlock(day.messageCount, maxMsg)}</Text>
            ))}
          </Box>
        ))}
      </Box>
    </Box>
  );
}
