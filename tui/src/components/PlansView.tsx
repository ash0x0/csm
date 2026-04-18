import React, { useState, useEffect } from 'react';
import { Box, Text, useInput } from 'ink';
import { Spinner } from '@inkjs/ui';
import { getPlans } from '../csm.js';
import type { PlanEntry } from '../types.js';

interface Props {
  onBack: () => void;
}

export function PlansView({ onBack }: Props) {
  const [plans, setPlans] = useState<PlanEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [cursor, setCursor] = useState(0);

  useEffect(() => {
    getPlans()
      .then(data => { setPlans(data); setLoading(false); })
      .catch(() => setLoading(false));
  }, []);

  useInput((input, key) => {
    if (key.escape || input === 'q') { onBack(); return; }
    if (key.upArrow) setCursor(c => Math.max(0, c - 1));
    if (key.downArrow) setCursor(c => Math.min(plans.length - 1, c + 1));
  });

  return (
    <Box flexDirection="column" padding={1}>
      <Box marginBottom={1}>
        <Text bold>Plans</Text>
        <Text dimColor>  (esc to go back)</Text>
      </Box>
      {loading && <Spinner label="Loading plans..." />}
      {!loading && plans.length === 0 && <Text dimColor>No plans found.</Text>}
      {plans.map((plan, i) => (
        <Box key={plan.slug} gap={2}>
          <Text color={i === cursor ? 'green' : undefined} bold={i === cursor}>
            {i === cursor ? '>' : ' '} {plan.title.length > 50 ? plan.title.slice(0, 47) + '...' : plan.title}
          </Text>
          <Text dimColor>{plan.slug}</Text>
          {plan.session_id && <Text dimColor>({plan.session_id})</Text>}
        </Box>
      ))}
    </Box>
  );
}
