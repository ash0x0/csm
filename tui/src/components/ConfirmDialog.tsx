import React from 'react';
import { Box, Text } from 'ink';
import { ConfirmInput } from '@inkjs/ui';

interface Props {
  message: string;
  onConfirm: () => void;
  onCancel: () => void;
}

export function ConfirmDialog({ message, onConfirm, onCancel }: Props) {
  return (
    <Box borderStyle="round" borderColor="red" paddingX={2} paddingY={1} flexDirection="column" gap={1}>
      <Text color="red" bold>⚠ Confirm</Text>
      <Text>{message}</Text>
      <Box gap={1}>
        <Text dimColor>Continue? </Text>
        <ConfirmInput
          defaultChoice="cancel"
          onConfirm={onConfirm}
          onCancel={onCancel}
        />
      </Box>
    </Box>
  );
}
