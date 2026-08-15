import React, { useCallback, useEffect, useState } from 'react';
import { ActivityIndicator, Alert, ScrollView, Text, TouchableOpacity, View } from 'react-native';
import { useFocusEffect } from '@react-navigation/native';

import { ApiError } from '../api/client';
import { formatMinorUnits } from '../utils/money';
import { listPhotos, listTransactions, purgeSyncedTransactions } from '../db/repo';
import { runSync } from '../sync/worker';
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

    const refresh = useCallback(async () => {
        const [pendingTransactions, allPhotos] = await Promise.all([
            listTransactions(['pending', 'syncing', 'failed']),
            listPhotos(['pending', 'submitted', 'needs_review'])
        ]);

        setTransactions(pendingTransactions);
        setPhotos(allPhotos);
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

            <TouchableOpacity onPress={() => void signOut()} style={{ padding: spacing.md, alignItems: 'center' }}>
                <Text style={styles.subtitle}>Signed in as {session?.username} — disconnect</Text>
            </TouchableOpacity>
        </ScrollView>
    );
}

function Counter({ value, caption, color }: { value: number; caption: string; color?: string }): React.ReactElement {
    return (
        <View style={{ alignItems: 'center', flex: 1 }}>
            <Text style={{ fontSize: 28, fontWeight: '700', color: color ?? colors.text }}>{value}</Text>
            <Text style={styles.subtitle}>{caption}</Text>
        </View>
    );
}
