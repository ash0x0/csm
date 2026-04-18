import React from 'react';
import { Box, Text } from 'ink';

interface Props {
  multiSelectCount: number;
  showHelp: boolean;
}

export function HelpBar({ multiSelectCount, showHelp }: Props) {
  if (showHelp) {
    return (
      <Box flexDirection="column" borderStyle="single" paddingX={1}>
        <Text bold>Keybindings</Text>
        <Box gap={4}>
          <Box flexDirection="column">
            <Text><Text color="yellow">↑↓</Text>  navigate</Text>
            <Text><Text color="yellow">space</Text>  select (multi)</Text>
            <Text><Text color="yellow">/</Text>    search</Text>
            <Text><Text color="yellow">enter</Text> open/merge</Text>
            <Text><Text color="yellow">esc</Text>  back/quit</Text>
          </Box>
          <Box flexDirection="column">
            <Text><Text color="yellow">d</Text>  delete</Text>
            <Text><Text color="yellow">o</Text>  move</Text>
            <Text><Text color="yellow">b</Text>  clone</Text>
            <Text><Text color="yellow">f</Text>  diff</Text>
            <Text><Text color="yellow">t</Text>  tasks</Text>
          </Box>
          <Box flexDirection="column">
            <Text><Text color="yellow">l</Text>  timeline</Text>
            <Text><Text color="yellow">p</Text>  plans</Text>
            <Text><Text color="yellow">a</Text>  activity</Text>
            <Text><Text color="yellow">?</Text>  toggle help</Text>
            <Text><Text color="yellow">q</Text>  quit</Text>
          </Box>
        </Box>
      </Box>
    );
  }

  const hint = multiSelectCount > 0
    ? `${multiSelectCount} selected — enter:merge  f:diff  esc:clear`
    : '↑↓ nav  space select  enter open  d del  o move  b clone  / search  ? help  q quit';

  return (
    <Box paddingX={1}>
      <Text dimColor>{hint}</Text>
    </Box>
  );
}
