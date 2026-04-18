import { describe, it, expect, vi, beforeEach } from 'vitest';

vi.mock('execa', () => ({
  execa: vi.fn(),
}));

import { execa } from 'execa';
import { init, listSessions, deleteSession, cloneSession } from '../csm.js';

const mockExeca = vi.mocked(execa);

beforeEach(() => {
  vi.clearAllMocks();
  init('/usr/bin/csm', '/home/user/.claude');
});

describe('listSessions', () => {
  it('parses JSON array from csm list --json', async () => {
    const sessions = [{ id: 'abc', short_id: 'abc12345', title: 'Test' }];
    mockExeca.mockResolvedValueOnce({ stdout: JSON.stringify(sessions) } as any);

    const result = await listSessions();

    expect(mockExeca).toHaveBeenCalledWith(
      '/usr/bin/csm',
      ['--claude-dir', '/home/user/.claude', 'list', '--json'],
      { reject: true }
    );
    expect(result).toEqual(sessions);
  });

  it('throws on subprocess failure', async () => {
    mockExeca.mockRejectedValueOnce(new Error('exit 1'));
    await expect(listSessions()).rejects.toThrow('exit 1');
  });
});

describe('deleteSession', () => {
  it('calls csm rm --force <id>', async () => {
    mockExeca.mockResolvedValueOnce({ stdout: '' } as any);
    await deleteSession('abc12345');
    expect(mockExeca).toHaveBeenCalledWith(
      '/usr/bin/csm',
      ['--claude-dir', '/home/user/.claude', 'rm', '--force', 'abc12345'],
      { reject: true }
    );
  });
});

describe('cloneSession', () => {
  it('extracts new short ID from clone output', async () => {
    mockExeca.mockResolvedValueOnce({
      stdout: 'Cloned abc12345 → def67890\nResume with: claude --resume def67890',
    } as any);
    const id = await cloneSession('abc12345');
    expect(id).toBe('def67890');
  });
});
