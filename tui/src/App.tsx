import React, { useState, useCallback, useMemo } from 'react';
import { Box, Text, useInput, useApp, useStdout } from 'ink';
import { Spinner } from '@inkjs/ui';
import { SessionList, buildRows } from './components/SessionList.js';
import { PreviewPanel } from './components/PreviewPanel.js';
import { SearchBar } from './components/SearchBar.js';
import { HelpBar } from './components/HelpBar.js';
import { ConfirmDialog } from './components/ConfirmDialog.js';
import { TimelineView } from './components/TimelineView.js';
import { TasksView } from './components/TasksView.js';
import { PlansView } from './components/PlansView.js';
import { ActivityView } from './components/ActivityView.js';
import { MoveView } from './components/MoveView.js';
import { DiffView } from './components/DiffView.js';
import { MergePreviewDialog } from './components/MergePreviewDialog.js';
import { useSessions } from './hooks/useSessions.js';
import { usePreview } from './hooks/usePreview.js';
import { deleteSession, cloneSession, mergeSession, dryRunMerge, moveSession, type MergeResult, type DryRunResult } from './csm.js';
import type { SessionMeta } from './types.js';

type Screen =
  | { type: 'main' }
  | { type: 'timeline'; session: SessionMeta }
  | { type: 'tasks'; session: SessionMeta }
  | { type: 'plans' }
  | { type: 'activity' }
  | { type: 'confirm-delete'; session: SessionMeta }
  | { type: 'move'; session: SessionMeta }
  | { type: 'diff'; sessionA: SessionMeta; sessionB: SessionMeta }
  | { type: 'merge-preview'; ids: string[]; preview: DryRunResult };

export function AppWithInput() {
  const { exit } = useApp();
  const { stdout } = useStdout();
  const termWidth = stdout?.columns ?? 120;
  const termHeight = stdout?.rows ?? 40;

  const [screen, setScreen] = useState<Screen>({ type: 'main' });
  const [searchQuery, setSearchQuery] = useState('');
  const [isSearchActive, setIsSearchActive] = useState(false);
  const [cursor, setCursor] = useState(0);
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());
  const [showHelp, setShowHelp] = useState(false);
  const [statusMsg, setStatusMsg] = useState<string | null>(null);
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(new Set());

  const { sessions, loading, error, refresh } = useSessions(searchQuery);

  const rows = useMemo(() => buildRows(sessions, collapsedGroups), [sessions, collapsedGroups]);
  const currentRow = rows[cursor] ?? null;
  const currentSession = currentRow?.type === 'session' ? currentRow.session : null;

  const { events: previewEvents, loading: previewLoading, error: previewError } = usePreview(
    screen.type === 'main' ? currentSession : null
  );

  const showStatus = useCallback((msg: string) => {
    setStatusMsg(msg);
    setTimeout(() => setStatusMsg(null), 2500);
  }, []);

  const toggleSelect = useCallback((id: string) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }, []);

  const toggleCollapse = useCallback((project: string) => {
    setCollapsedGroups(prev => {
      const next = new Set(prev);
      if (next.has(project)) next.delete(project); else next.add(project);
      return next;
    });
  }, []);

  const isMainScreen = screen.type === 'main';

  useInput((input, key) => {
    if (isSearchActive) {
      if (key.escape) {
        setIsSearchActive(false);
        setSearchQuery('');
        setCursor(0);
      }
      return;
    }

    if (!isMainScreen) return;

    if (input === 'q' || key.escape) {
      if (selectedIds.size > 0) { setSelectedIds(new Set()); return; }
      exit();
      return;
    }
    if (input === '/') { setIsSearchActive(true); return; }
    if (input === '?') { setShowHelp(h => !h); return; }

    if (key.upArrow) { setCursor(c => Math.max(0, c - 1)); return; }
    if (key.downArrow) { setCursor(c => Math.min(rows.length - 1, c + 1)); return; }

    if (input === ' ') {
      if (currentRow?.type === 'header') {
        toggleCollapse(currentRow.project);
      } else if (currentSession) {
        toggleSelect(currentSession.short_id);
      }
      return;
    }

    if (key.return) {
      if (currentRow?.type === 'header') {
        toggleCollapse(currentRow.project);
        return;
      }
      const selected = sessions.filter(s => selectedIds.has(s.short_id));
      if (selected.length >= 2) {
        const ids = selected.map(s => s.short_id);
        dryRunMerge(ids).then(preview => {
          setScreen({ type: 'merge-preview', ids, preview });
        }).catch(e => showStatus(`Merge preview failed: ${e}`));
      }
      return;
    }

    if (input === 'd' && currentSession) {
      setScreen({ type: 'confirm-delete', session: currentSession });
      return;
    }

    if (input === 'b' && currentSession) {
      cloneSession(currentSession.short_id).then(newId => {
        refresh();
        showStatus(`Cloned → ${newId}`);
      }).catch(e => showStatus(`Clone failed: ${e}`));
      return;
    }

    if (input === 'l' && currentSession) {
      setScreen({ type: 'timeline', session: currentSession });
      return;
    }

    if (input === 't' && currentSession) {
      setScreen({ type: 'tasks', session: currentSession });
      return;
    }

    if (input === 'p') {
      setScreen({ type: 'plans' });
      return;
    }

    if (input === 'a') {
      setScreen({ type: 'activity' });
      return;
    }

    if (input === 'f') {
      const selected = sessions.filter(s => selectedIds.has(s.short_id));
      if (selected.length === 2) {
        setScreen({ type: 'diff', sessionA: selected[0], sessionB: selected[1] });
      } else {
        showStatus('Select exactly 2 sessions for diff (press space)');
      }
      return;
    }

    if (input === 'o' && currentSession) {
      setScreen({ type: 'move', session: currentSession });
      return;
    }
  }, { isActive: isMainScreen && !isSearchActive });

  const listHeight = Math.max(5, termHeight - 6);
  const previewWidth = Math.floor(termWidth * 0.4);
  const listWidth = termWidth - previewWidth - 3;

  if (screen.type === 'timeline') {
    return <TimelineView session={screen.session} onBack={() => setScreen({ type: 'main' })} />;
  }
  if (screen.type === 'tasks') {
    return <TasksView session={screen.session} onBack={() => setScreen({ type: 'main' })} />;
  }
  if (screen.type === 'plans') {
    return <PlansView onBack={() => setScreen({ type: 'main' })} />;
  }
  if (screen.type === 'activity') {
    return <ActivityView onBack={() => setScreen({ type: 'main' })} />;
  }
  if (screen.type === 'confirm-delete') {
    const s = screen.session;
    return (
      <ConfirmDialog
        message={`Delete session "${s.title}" (${s.short_id})?`}
        onConfirm={async () => {
          await deleteSession(s.short_id);
          setCursor(c => Math.max(0, c - 1));
          refresh();
          setScreen({ type: 'main' });
          showStatus(`Deleted ${s.short_id}`);
        }}
        onCancel={() => setScreen({ type: 'main' })}
      />
    );
  }
  if (screen.type === 'move') {
    return (
      <MoveView
        session={screen.session}
        onMove={async dest => {
          await moveSession(screen.session.short_id, dest);
          refresh();
          setScreen({ type: 'main' });
          showStatus(`Moved ${screen.session.short_id} → ${dest.split('/').pop()}`);
        }}
        onCancel={() => setScreen({ type: 'main' })}
      />
    );
  }
  if (screen.type === 'diff') {
    return (
      <DiffView
        sessionA={screen.sessionA}
        sessionB={screen.sessionB}
        onBack={() => setScreen({ type: 'main' })}
      />
    );
  }
  if (screen.type === 'merge-preview') {
    const { ids, preview } = screen;
    return (
      <MergePreviewDialog
        ids={ids}
        preview={preview}
        onConfirm={() => {
          mergeSession(ids).then((result: MergeResult) => {
            refresh();
            setSelectedIds(new Set());
            setScreen({ type: 'main' });
            const strategyInfo = result.strategy ? ` (${result.strategy}, ${result.totalEvents} events)` : '';
            showStatus(`Merged → ${result.newId.slice(0, 8)}${strategyInfo}`);
          }).catch(e => {
            setScreen({ type: 'main' });
            showStatus(`Merge failed: ${e}`);
          });
        }}
        onCancel={() => setScreen({ type: 'main' })}
      />
    );
  }

  return (
    <Box flexDirection="column" height={termHeight}>
      <SearchBar
        value={searchQuery}
        isActive={isSearchActive}
        onChange={q => { setSearchQuery(q); setCursor(0); }}
        onSubmit={() => setIsSearchActive(false)}
        onCancel={() => { setIsSearchActive(false); setSearchQuery(''); setCursor(0); }}
        totalCount={sessions.length}
      />
      <Box flexGrow={1} overflow="hidden">
        <Box width={listWidth} flexDirection="column">
          {loading && <Box padding={1}><Spinner label="Loading sessions..." /></Box>}
          {error && <Box padding={1}><Text color="red">Error: {error}</Text></Box>}
          {!loading && !error && (
            <SessionList
              sessions={sessions}
              collapsedGroups={collapsedGroups}
              onToggleCollapse={toggleCollapse}
              selectedIndex={cursor}
              selectedIds={selectedIds}
              onIndexChange={setCursor}
              onToggleSelect={toggleSelect}
              height={listHeight}
            />
          )}
        </Box>
        <Box width={1} flexDirection="column">
          {Array.from({ length: listHeight }, (_, i) => (
            <Text key={i} color="blue">│</Text>
          ))}
        </Box>
        <Box width={previewWidth} overflow="hidden">
          <PreviewPanel
            session={currentSession}
            events={previewEvents}
            loading={previewLoading}
            error={previewError}
            width={previewWidth}
            height={listHeight}
          />
        </Box>
      </Box>
      {statusMsg ? (
        <Box paddingX={1}><Text color="green">{statusMsg}</Text></Box>
      ) : (
        <HelpBar multiSelectCount={selectedIds.size} showHelp={showHelp} />
      )}
    </Box>
  );
}
