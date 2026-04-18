import React from 'react';
import { Box, Text } from 'ink';
import { TextInput } from '@inkjs/ui';

interface Props {
  value: string;
  isActive: boolean;
  onChange: (value: string) => void;
  onSubmit: () => void;
  onCancel: () => void;
  totalCount: number;
}

export function SearchBar({ value, isActive, onChange, onCancel, totalCount }: Props) {
  return (
    <Box borderStyle="single" paddingX={1}>
      <Text dimColor>/ </Text>
      {isActive ? (
        <TextInput
          placeholder="Search sessions..."
          defaultValue={value}
          onChange={onChange}
          onSubmit={onCancel}
        />
      ) : (
        <Text>{value || <Text dimColor>Search sessions... (press / to search)</Text>}</Text>
      )}
      <Box flexGrow={1} />
      <Text dimColor> {totalCount} sessions</Text>
    </Box>
  );
}
