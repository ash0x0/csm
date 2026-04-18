import React from 'react';
import { Box, Text } from 'ink';

interface Props {
  multiSelectCount: number;
  showHelp: boolean;
}

export function HelpBar({ multiSelectCount, showHelp }: Props) {
  if (showHelp) {
    return (
      <Box flexDirection="column" borderStyle="single" borderColor="cyan" paddingX={1}>
        <Text bold>Keybindings</Text>
        <Box gap={4}>
          <Box flexDirection="column">
            <Text><Text color="cyan" bold>↑↓</Text>  navigate</Text>
            <Text><Text color="cyan" bold>space</Text>  select / collapse</Text>
            <Text><Text color="cyan" bold>/</Text>    search</Text>
            <Text><Text color="cyan" bold>enter</Text> open/merge/collapse</Text>
            <Text><Text color="cyan" bold>esc</Text>  back/quit</Text>
          </Box>
          <Box flexDirection="column">
            <Text><Text color="cyan" bold>d</Text>  delete</Text>
            <Text><Text color="cyan" bold>o</Text>  move</Text>
            <Text><Text color="cyan" bold>b</Text>  clone</Text>
            <Text><Text color="cyan" bold>f</Text>  diff</Text>
            <Text><Text color="cyan" bold>t</Text>  tasks</Text>
          </Box>
          <Box flexDirection="column">
            <Text><Text color="cyan" bold>l</Text>  timeline</Text>
            <Text><Text color="cyan" bold>p</Text>  plans</Text>
            <Text><Text color="cyan" bold>a</Text>  activity</Text>
            <Text><Text color="cyan" bold>?</Text>  toggle help</Text>
            <Text><Text color="cyan" bold>q</Text>  quit</Text>
          </Box>
        </Box>
      </Box>
    );
  }

  if (multiSelectCount > 0) {
    return (
      <Box paddingX={1}>
        <Text color="magenta" bold>{multiSelectCount} selected</Text>
        <Text> — enter:merge  f:diff  esc:clear</Text>
      </Box>
    );
  }

  return (
    <Box paddingX={1}>
      <Text>↑↓ nav  space select  enter open  d del  o move  b clone  / search  ? help  q quit</Text>
    </Box>
  );
}
