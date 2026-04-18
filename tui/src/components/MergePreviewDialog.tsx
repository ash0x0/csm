import React from 'react';
import { Box, Text, useInput } from 'ink';
import type { DryRunResult } from '../csm.js';

interface Props {
  ids: string[];
  preview: DryRunResult;
  onConfirm: () => void;
  onCancel: () => void;
}

export function MergePreviewDialog({ ids, preview, onConfirm, onCancel }: Props) {
  useInput((input, key) => {
    if (input === 'y' || key.return) { onConfirm(); return; }
    if (input === 'n' || key.escape) { onCancel(); return; }
  });

  return (
    <Box borderStyle="round" borderColor="cyan" paddingX={2} paddingY={1} flexDirection="column" gap={1}>
      <Text color="cyan" bold>Merge Preview</Text>

      <Box flexDirection="column">
        <Text>Sessions: <Text bold>{ids.join(', ')}</Text></Text>
        <Text>Strategy: <Text bold color="yellow">{preview.strategy}</Text></Text>
      </Box>

      <Box flexDirection="column">
        <Text>Shared events:    <Text bold>{preview.sharedCount}</Text></Text>
        <Text>Session A unique: <Text bold>{preview.branchAOnly}</Text></Text>
        <Text>Session B unique: <Text bold>{preview.branchBOnly}</Text></Text>
        <Text>Total events:     <Text bold color="green">{preview.totalEvents}</Text></Text>
      </Box>

      {preview.warnings.length > 0 && (
        <Box flexDirection="column">
          <Text color="yellow" bold>⚠ Warnings</Text>
          {preview.warnings.map((w, i) => (
            <Text key={i} color="yellow">  {w}</Text>
          ))}
        </Box>
      )}

      <Box gap={2}>
        <Text dimColor>Confirm merge?</Text>
        <Text color="green" bold>y / Enter</Text>
        <Text dimColor>to confirm</Text>
        <Text color="red" bold>n / Esc</Text>
        <Text dimColor>to cancel</Text>
      </Box>
    </Box>
  );
}
