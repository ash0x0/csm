import React, { useState, useEffect } from 'react';
import { Box, Text, useInput } from 'ink';
import { Spinner } from '@inkjs/ui';
import { getTasks } from '../csm.js';
import type { SessionMeta, Task } from '../types.js';

interface Props {
  session: SessionMeta;
  onBack: () => void;
}

const STATUS_COLOR: Record<string, string> = {
  completed: 'green',
  in_progress: 'yellow',
  pending: 'white',
};

const STATUS_ICON: Record<string, string> = {
  completed: '✓',
  in_progress: '●',
  pending: '○',
};

export function TasksView({ session, onBack }: Props) {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    getTasks(session.short_id)
      .then(data => { setTasks(data); setLoading(false); })
      .catch(() => setLoading(false));
  }, [session.short_id]);

  useInput((input, key) => {
    if (key.escape || input === 'q') onBack();
  });

  return (
    <Box flexDirection="column" padding={1}>
      <Box marginBottom={1}>
        <Text bold>Tasks: </Text>
        <Text color="cyan">{session.title}</Text>
        <Text dimColor>  (esc to go back)</Text>
      </Box>
      {loading && <Spinner label="Loading tasks..." />}
      {!loading && tasks.length === 0 && <Text dimColor>No tasks for this session.</Text>}
      {tasks.map(task => (
        <Box key={task.id} gap={2}>
          <Text color={STATUS_COLOR[task.status] ?? 'white'}>{STATUS_ICON[task.status] ?? '?'}</Text>
          <Text>{task.subject}</Text>
          <Text dimColor>{task.status}</Text>
        </Box>
      ))}
    </Box>
  );
}
