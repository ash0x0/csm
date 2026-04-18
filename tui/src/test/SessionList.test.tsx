import React from 'react';
import { describe, it, expect } from 'vitest';
import { render } from 'ink-testing-library';
import { SessionList } from '../components/SessionList.js';
import type { SessionMeta } from '../types.js';

const sessions: SessionMeta[] = [
  {
    id: 'aaa-bbb', short_id: 'aaa-bbb1', title: 'First session',
    project: '/home/user/code/proj', branch: 'main', messages: 10,
    created: '2026-01-01T00:00:00Z', modified: '2026-01-02T00:00:00Z',
    file_size: 1000, file_path: '/path/to/file.jsonl', is_active: false,
  },
  {
    id: 'ccc-ddd', short_id: 'ccc-ddd1', title: 'Active session',
    project: '/home/user/code/proj', branch: 'main', messages: 5,
    created: '2026-01-01T00:00:00Z', modified: '2026-01-03T00:00:00Z',
    file_size: 500, file_path: '/path/to/file2.jsonl', is_active: true,
  },
];

describe('SessionList', () => {
  it('renders session titles', () => {
    const { lastFrame } = render(
      <SessionList
        sessions={sessions}
        selectedIndex={0}
        selectedIds={new Set()}
        onIndexChange={() => {}}
        onToggleSelect={() => {}}
        height={20}
      />
    );
    expect(lastFrame()).toContain('First session');
    expect(lastFrame()).toContain('Active session');
  });

  it('marks active sessions', () => {
    const { lastFrame } = render(
      <SessionList
        sessions={sessions}
        selectedIndex={0}
        selectedIds={new Set()}
        onIndexChange={() => {}}
        onToggleSelect={() => {}}
        height={20}
      />
    );
    expect(lastFrame()).toContain('*'); // active marker
  });

  it('highlights the selected row', () => {
    const { lastFrame } = render(
      <SessionList
        sessions={sessions}
        selectedIndex={1}
        selectedIds={new Set()}
        onIndexChange={() => {}}
        onToggleSelect={() => {}}
        height={20}
      />
    );
    expect(lastFrame()).toContain('>'); // cursor marker
  });

  it('groups sessions by project', () => {
    const { lastFrame } = render(
      <SessionList
        sessions={sessions}
        selectedIndex={0}
        selectedIds={new Set()}
        onIndexChange={() => {}}
        onToggleSelect={() => {}}
        height={20}
      />
    );
    expect(lastFrame()).toContain('proj'); // project header
  });
});
