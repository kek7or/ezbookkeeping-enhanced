import React, { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Alert, FlatList, Text, TouchableOpacity, View } from 'react-native';
import * as Sharing from 'expo-sharing';

import { clearLogs, formatTimestamp, readLogs, writeLogFile } from '../utils/logger';
import { colors, spacing, styles } from '../ui/theme';

import type { LogEntry, LogLevel } from '../utils/logger';
import type { ScreenProps } from '../navigation/types';

/**
 * The log viewer.
 *
 * Newest first, because the thing you came to look at is the thing that just
 * happened. Details stay collapsed until tapped: a stack trace is worth having
 * but not worth scrolling past forty times to reach the line below it.
 */

interface Filter {
    key: string;
    label: string;
    levels?: LogLevel[];
}

const FILTERS: Filter[] = [
    { key: 'all', label: 'All' },
    { key: 'problems', label: 'Problems', levels: ['warn', 'error'] },
    { key: 'errors', label: 'Errors', levels: ['error'] }
];

const LEVEL_COLORS: Record<LogLevel, string> = {
    debug: colors.textMuted,
    info: colors.text,
    warn: colors.warning,
    error: colors.danger
};

export function LogsScreen({ navigation }: ScreenProps<'Logs'>): React.ReactElement {
    const [entries, setEntries] = useState<LogEntry[]>([]);
    const [filterKey, setFilterKey] = useState('all');
    const [expanded, setExpanded] = useState<Set<number>>(new Set());
    const [loading, setLoading] = useState(true);
    const [busy, setBusy] = useState(false);

    const filter = FILTERS.find((candidate) => candidate.key === filterKey) ?? FILTERS[0];

    const refresh = useCallback(async () => {
        setLoading(true);

        try {
            setEntries(await readLogs({ levels: filter.levels }));
        } catch (error) {
            Alert.alert('Could not read the log', error instanceof Error ? error.message : String(error));
        } finally {
            setLoading(false);
        }
    }, [filter.levels]);

    useEffect(() => {
        void refresh();
    }, [refresh]);

    useEffect(() => {
        navigation.setOptions({
            headerRight: () => (
                <TouchableOpacity onPress={() => void refresh()} disabled={loading}>
                    <Text style={{ color: colors.primary, fontSize: 15, fontWeight: '600' }}>Refresh</Text>
                </TouchableOpacity>
            )
        });
    }, [navigation, refresh, loading]);

    function toggle(id: number): void {
        setExpanded((current) => {
            const next = new Set(current);

            if (next.has(id)) {
                next.delete(id);
            } else {
                next.add(id);
            }

            return next;
        });
    }

    async function handleExport(): Promise<void> {
        if (!entries.length) {
            Alert.alert('Nothing to export', 'There are no log entries matching this filter.');
            return;
        }

        setBusy(true);

        try {
            const file = await writeLogFile(entries);

            // Sharing is how a file leaves an Android app: the share sheet lets
            // the user drop it into Files, Drive, an email, or a chat. Writing
            // to a public folder instead would need a storage permission the
            // app has no other use for.
            if (await Sharing.isAvailableAsync()) {
                await Sharing.shareAsync(file.uri, {
                    mimeType: 'text/plain',
                    UTI: 'public.plain-text',
                    dialogTitle: 'ezbookkeeping log'
                });
            } else {
                Alert.alert('Log saved', `Sharing is unavailable on this device. The file is at:\n\n${file.uri}`);
            }
        } catch (error) {
            Alert.alert('Could not export the log', error instanceof Error ? error.message : String(error));
        } finally {
            setBusy(false);
        }
    }

    function handleClear(): void {
        Alert.alert('Clear the log?', 'Every entry is deleted. This cannot be undone.', [
            { text: 'Cancel', style: 'cancel' },
            {
                text: 'Clear',
                style: 'destructive',
                onPress: () => {
                    void (async () => {
                        try {
                            await clearLogs();
                            await refresh();
                        } catch (error) {
                            Alert.alert(
                                'Could not clear the log',
                                error instanceof Error ? error.message : String(error)
                            );
                        }
                    })();
                }
            }
        ]);
    }

    return (
        <View style={styles.screen}>
            <View
                style={{
                    padding: spacing.lg,
                    gap: spacing.md,
                    borderBottomWidth: 1,
                    borderBottomColor: colors.border,
                    backgroundColor: colors.surface
                }}
            >
                <View style={styles.row}>
                    {FILTERS.map((candidate) => {
                        const active = candidate.key === filterKey;

                        return (
                            <TouchableOpacity
                                key={candidate.key}
                                onPress={() => setFilterKey(candidate.key)}
                                style={{
                                    paddingVertical: spacing.sm,
                                    paddingHorizontal: spacing.md,
                                    borderRadius: 999,
                                    borderWidth: 1,
                                    borderColor: active ? colors.primary : colors.border,
                                    backgroundColor: active ? colors.primary : colors.surface
                                }}
                            >
                                <Text
                                    style={{
                                        color: active ? colors.primaryText : colors.text,
                                        fontWeight: '600',
                                        fontSize: 14
                                    }}
                                >
                                    {candidate.label}
                                </Text>
                            </TouchableOpacity>
                        );
                    })}
                </View>

                <View style={styles.row}>
                    <TouchableOpacity
                        style={[styles.button, { flex: 1 }, (busy || !entries.length) && styles.buttonDisabled]}
                        onPress={() => void handleExport()}
                        disabled={busy || !entries.length}
                    >
                        <Text style={styles.buttonText}>
                            {busy ? 'Preparing…' : `Export ${entries.length} entr${entries.length === 1 ? 'y' : 'ies'}`}
                        </Text>
                    </TouchableOpacity>
                    <TouchableOpacity
                        style={[styles.button, styles.buttonSecondary]}
                        onPress={handleClear}
                        disabled={busy}
                    >
                        <Text style={[styles.buttonText, { color: colors.danger }]}>Clear</Text>
                    </TouchableOpacity>
                </View>
            </View>

            {loading ? (
                <View style={{ padding: spacing.xl, alignItems: 'center' }}>
                    <ActivityIndicator color={colors.primary} />
                </View>
            ) : (
                <FlatList
                    data={entries}
                    keyExtractor={(entry) => String(entry.id)}
                    contentContainerStyle={{ padding: spacing.lg, gap: spacing.sm }}
                    ListEmptyComponent={
                        <Text style={[styles.subtitle, { textAlign: 'center', marginTop: spacing.xl }]}>
                            Nothing logged yet. Upload or sync, then come back.
                        </Text>
                    }
                    renderItem={({ item }) => (
                        <LogRow entry={item} expanded={expanded.has(item.id)} onPress={() => toggle(item.id)} />
                    )}
                />
            )}
        </View>
    );
}

function LogRow({
    entry,
    expanded,
    onPress
}: {
    entry: LogEntry;
    expanded: boolean;
    onPress: () => void;
}): React.ReactElement {
    return (
        <TouchableOpacity
            onPress={onPress}
            disabled={!entry.detail}
            style={{
                backgroundColor: colors.surface,
                borderRadius: 8,
                borderWidth: 1,
                borderColor: colors.border,
                borderLeftWidth: 3,
                borderLeftColor: LEVEL_COLORS[entry.level],
                padding: spacing.md,
                gap: spacing.xs
            }}
        >
            <View style={[styles.row, { justifyContent: 'space-between' }]}>
                <Text style={{ fontSize: 12, color: colors.textMuted }}>
                    {formatTimestamp(entry.at)} · {entry.scope}
                </Text>
                <Text style={{ fontSize: 12, fontWeight: '700', color: LEVEL_COLORS[entry.level] }}>
                    {entry.level.toUpperCase()}
                </Text>
            </View>

            <Text style={[styles.body, { color: LEVEL_COLORS[entry.level] }]}>{entry.message}</Text>

            {entry.detail ? (
                expanded ? (
                    <Text style={{ fontSize: 12, color: colors.textMuted, fontFamily: 'monospace' }}>
                        {entry.detail}
                    </Text>
                ) : (
                    <Text style={{ fontSize: 12, color: colors.primary }}>Tap for details</Text>
                )
            ) : null}
        </TouchableOpacity>
    );
}
