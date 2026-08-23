import React, { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Alert, ScrollView, Text, TouchableOpacity, View } from 'react-native';
import { useFocusEffect } from '@react-navigation/native';

import { ApiError } from '../api/client';
import { formatMinorUnits } from '../utils/money';
import { getMeta, listPhotos, listTransactions, purgeSyncedTransactions } from '../db/repo';
import { logger } from '../utils/logger';
import { pullReferenceData, runSync } from '../sync/worker';
import { useApp } from '../state/AppContext';
import { colors, spacing, styles } from '../ui/theme';

import type { LocalTransaction, Photo } from '../db/repo';
import type { ScreenProps } from '../navigation/types';
import type { SyncProgress } from '../sync/worker';

export function HomeScreen({ navigation }: ScreenProps<'Home'>): React.ReactElement {
    const { session, client, signOut, dataVersion, notifyDataChanged } = useApp();
    const [transactions, setTransactions] = useState<LocalTransaction[]>([]);
    const [photos, setPhotos] = useState<Photo[]>([]);
    const [progress, setProgress] = useState<SyncProgress | null>(null);
    const [pullingReferenceData, setPullingReferenceData] = useState(false);
    const [lastPullAt, setLastPullAt] = useState<number | null>(null);

    const refresh = useCallback(async () => {
        const [pendingTransactions, allPhotos, pulledAt] = await Promise.all([
            listTransactions(['pending', 'syncing', 'failed']),
            listPhotos(['pending', 'submitted', 'needs_review']),
            getMeta('last_pull_at')
        ]);

        setTransactions(pendingTransactions);
        setPhotos(allPhotos);
        setLastPullAt(pulledAt ? Number(pulledAt) : null);
    }, []);

    useFocusEffect(
        useCallback(() => {
            void refresh();
        }, [refresh])
    );

    useEffect(() => {
        void refresh();
    }, [refresh, dataVersion]);

    const pendingTransactions = transactions.filter((t) => t.syncState === 'pending');
    const failedTransactions = transactions.filter((t) => t.syncState === 'failed');
    const pendingPhotos = photos.filter((p) => p.syncState === 'pending');
    // Handed over to the server's queue. Shown so the count is not confusing,
    // but there is nothing for the user to do about these.
    const submittedPhotos = photos.filter((p) => p.syncState === 'submitted');
    const reviewPhotos = photos.filter((p) => p.syncState === 'needs_review');

    // Submitted photos still justify a sync: it is how finished results come back.
    const nothingToUpload = !pendingTransactions.length && !pendingPhotos.length && !submittedPhotos.length;
    const syncing = progress !== null && progress.stage !== 'done';

    async function handleSyncCategories(): Promise<void> {
        if (!client) {
            return;
        }

        setPullingReferenceData(true);
        logger.info('ui', 'Category sync requested');

        try {
            const summary = await pullReferenceData(client);
            await refresh();
            // The transaction form reads categories and accounts straight from
            // SQLite, so it has to be told they changed underneath it.
            notifyDataChanged();

            Alert.alert(
                'Up to date',
                [
                    `${summary.expenseCategories} expense categor${summary.expenseCategories === 1 ? 'y' : 'ies'}`,
                    `${summary.incomeCategories} income categor${summary.incomeCategories === 1 ? 'y' : 'ies'}`,
                    `${summary.transferCategories} transfer categor${summary.transferCategories === 1 ? 'y' : 'ies'}`,
                    `${summary.accounts} account${summary.accounts === 1 ? '' : 's'}`,
                    `${summary.tags} tag${summary.tags === 1 ? '' : 's'}`
                ].join('\n')
            );
        } catch (error) {
            if (error instanceof ApiError && error.isAuthFailure) {
                Alert.alert('Signed out', 'The server rejected your credentials. Please connect again.', [
                    { text: 'OK', onPress: () => void signOut() }
                ]);
                return;
            }

            logger.error('ui', 'Category sync failed', error);
            Alert.alert(
                'Could not sync',
                `${error instanceof Error ? error.message : String(error)}\n\nOpen Logs for the details.`
            );
        } finally {
            setPullingReferenceData(false);
        }
    }

    async function handleUpload(): Promise<void> {
        if (!client) {
            return;
        }

        setProgress({ stage: 'pulling', message: 'Starting', fraction: null });

        try {
            const result = await runSync(client, setProgress);
            await purgeSyncedTransactions();
            await refresh();
            notifyDataChanged();

            const parts: string[] = [];

            if (result.transactionsPushed) {
                parts.push(`${result.transactionsPushed} transaction${result.transactionsPushed === 1 ? '' : 's'} uploaded`);
            }

            if (result.photosSubmitted) {
                parts.push(
                    `${result.photosSubmitted} receipt${result.photosSubmitted === 1 ? '' : 's'} sent for reading`
                );
            }

            if (result.photosReady) {
                parts.push(`${result.photosReady} receipt${result.photosReady === 1 ? '' : 's'} ready to review`);
            }

            if (result.stillProcessing) {
                parts.push(
                    `${result.stillProcessing} still being read — check back in a moment, no need to wait here`
                );
            }

            if (result.rejected.length) {
                parts.push(`${result.rejected.length} rejected — tap them to fix`);
            }

            if (!parts.length) {
                parts.push('Nothing needed uploading.');
            }

            const detail = result.errors.length ? `\n\n${result.errors.join('\n')}` : '';
            Alert.alert('Upload finished', `${parts.join('\n')}${detail}`);
        } catch (error) {
            if (error instanceof ApiError && error.isAuthFailure) {
                Alert.alert('Signed out', 'The server rejected your credentials. Please connect again.', [
                    { text: 'OK', onPress: () => void signOut() }
                ]);
                return;
            }

            logger.error('ui', 'Upload failed', error);
            Alert.alert('Upload failed', error instanceof Error ? error.message : String(error));
        } finally {
            setProgress(null);
            await refresh();
        }
    }

    return (
        <ScrollView style={styles.screen} contentContainerStyle={styles.content}>
            <View style={styles.card}>
                <Text style={styles.label}>Waiting to upload</Text>
                <View style={[styles.row, { justifyContent: 'space-between' }]}>
                    <Counter value={pendingTransactions.length} caption="transactions" />
                    <Counter value={pendingPhotos.length} caption="receipt photos" />
                    <Counter value={submittedPhotos.length} caption="being read" />
                    <Counter
                        value={reviewPhotos.length}
                        caption="to review"
                        color={reviewPhotos.length ? colors.warning : undefined}
                    />
                </View>

                {syncing ? (
                    <View style={[styles.row, { marginTop: spacing.sm }]}>
                        <ActivityIndicator color={colors.primary} />
                        <Text style={styles.subtitle}>
                            {progress.message}
                            {progress.fraction !== null ? ` (${Math.round(progress.fraction * 100)}%)` : ''}
                        </Text>
                    </View>
                ) : (
                    <TouchableOpacity
                        style={[styles.button, (nothingToUpload || !client) && styles.buttonDisabled, { marginTop: spacing.sm }]}
                        onPress={() => void handleUpload()}
                        disabled={nothingToUpload || !client}
                    >
                        <Text style={styles.buttonText}>Upload</Text>
                    </TouchableOpacity>
                )}
            </View>

            <View style={styles.row}>
                <TouchableOpacity
                    style={[styles.button, { flex: 1 }]}
                    onPress={() => navigation.navigate('AddTransaction')}
                >
                    <Text style={styles.buttonText}>Add expense</Text>
                </TouchableOpacity>
                <TouchableOpacity
                    style={[styles.button, styles.buttonSecondary, { flex: 1 }]}
                    onPress={() => navigation.navigate('Camera')}
                >
                    <Text style={[styles.buttonText, styles.buttonSecondaryText]}>Snap receipt</Text>
                </TouchableOpacity>
            </View>

            <View style={styles.card}>
                <Text style={styles.label}>Categories and accounts</Text>
                <Text style={styles.subtitle}>
                    {lastPullAt
                        ? `Last updated ${describeAge(lastPullAt)}.`
                        : 'Never updated on this device.'}{' '}
                    Add a category on the server, then sync to pick it here.
                </Text>

                {pullingReferenceData ? (
                    <View style={[styles.row, { marginTop: spacing.xs }]}>
                        <ActivityIndicator color={colors.primary} />
                        <Text style={styles.subtitle}>Fetching from the server</Text>
                    </View>
                ) : (
                    <TouchableOpacity
                        style={[
                            styles.button,
                            styles.buttonSecondary,
                            !client && styles.buttonDisabled,
                            { marginTop: spacing.xs }
                        ]}
                        onPress={() => void handleSyncCategories()}
                        disabled={!client}
                    >
                        <Text style={[styles.buttonText, styles.buttonSecondaryText]}>Sync categories</Text>
                    </TouchableOpacity>
                )}
            </View>

            {reviewPhotos.length ? (
                <TouchableOpacity style={styles.card} onPress={() => navigation.navigate('Review')}>
                    <Text style={styles.title}>
                        {reviewPhotos.length} receipt{reviewPhotos.length === 1 ? '' : 's'} to review
                    </Text>
                    <Text style={styles.subtitle}>Check what was read from them, then save.</Text>
                </TouchableOpacity>
            ) : null}

            {failedTransactions.length ? (
                <View style={styles.card}>
                    <Text style={[styles.label, { color: colors.danger }]}>Rejected by the server</Text>
                    {failedTransactions.map((transaction) => (
                        <TouchableOpacity
                            key={transaction.id}
                            onPress={() => navigation.navigate('AddTransaction', { transactionId: transaction.id })}
                        >
                            <Text style={styles.body}>{formatMinorUnits(transaction.sourceAmount)}</Text>
                            <Text style={styles.errorText}>{transaction.lastError ?? 'Unknown error'}</Text>
                        </TouchableOpacity>
                    ))}
                </View>
            ) : null}

            {pendingTransactions.length ? (
                <View style={styles.card}>
                    <Text style={styles.label}>Queued</Text>
                    {pendingTransactions.map((transaction) => (
                        <View key={transaction.id} style={[styles.row, { justifyContent: 'space-between' }]}>
                            <Text style={styles.body} numberOfLines={1}>
                                {transaction.comment || 'No description'}
                            </Text>
                            <Text style={styles.body}>{formatMinorUnits(transaction.sourceAmount)}</Text>
                        </View>
                    ))}
                </View>
            ) : null}

            <TouchableOpacity
                style={[styles.button, styles.buttonSecondary]}
                onPress={() => navigation.navigate('Logs')}
            >
                <Text style={[styles.buttonText, styles.buttonSecondaryText]}>View logs</Text>
            </TouchableOpacity>

            <TouchableOpacity onPress={() => void signOut()} style={{ padding: spacing.md, alignItems: 'center' }}>
                <Text style={styles.subtitle}>Signed in as {session?.username} — disconnect</Text>
            </TouchableOpacity>
        </ScrollView>
    );
}

/**
 * "3 minutes ago" rather than a timestamp: what matters about the last pull is
 * whether it was recent enough to trust, not exactly when it happened.
 */
function describeAge(at: number): string {
    const seconds = Math.max(0, Math.round((Date.now() - at) / 1000));

    if (seconds < 60) {
        return 'just now';
    }

    const minutes = Math.round(seconds / 60);

    if (minutes < 60) {
        return `${minutes} minute${minutes === 1 ? '' : 's'} ago`;
    }

    const hours = Math.round(minutes / 60);

    if (hours < 24) {
        return `${hours} hour${hours === 1 ? '' : 's'} ago`;
    }

    const days = Math.round(hours / 24);
    return `${days} day${days === 1 ? '' : 's'} ago`;
}

function Counter({ value, caption, color }: { value: number; caption: string; color?: string }): React.ReactElement {
    return (
        <View style={{ alignItems: 'center', flex: 1 }}>
            <Text style={{ fontSize: 28, fontWeight: '700', color: color ?? colors.text }}>{value}</Text>
            <Text style={styles.subtitle}>{caption}</Text>
        </View>
    );
}
