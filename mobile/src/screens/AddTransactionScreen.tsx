import React, { useEffect, useMemo, useState } from 'react';
import { Alert, KeyboardAvoidingView, Platform, ScrollView, Text, TextInput, TouchableOpacity, View } from 'react-native';

import {
    CATEGORY_TYPE_EXPENSE,
    CATEGORY_TYPE_INCOME,
    TRANSACTION_TYPE_EXPENSE,
    TRANSACTION_TYPE_INCOME
} from '../api/types';
import { deleteTransaction, getPhoto, insertTransaction, listAccounts, listCategories, markPhotoState } from '../db/repo';
import { formatMinorUnits, parseAmountToMinorUnits } from '../utils/money';
import { useApp } from '../state/AppContext';
import { colors, spacing, styles } from '../ui/theme';

import type { Account, Category } from '../db/repo';
import type { ScreenProps } from '../navigation/types';
import type { TransactionType } from '../api/types';

export function AddTransactionScreen({ navigation, route }: ScreenProps<'AddTransaction'>): React.ReactElement {
    const { session, notifyDataChanged } = useApp();
    const photoId = route.params?.photoId;
    const retryTransactionId = route.params?.transactionId;

    const [categories, setCategories] = useState<Category[]>([]);
    const [accounts, setAccounts] = useState<Account[]>([]);
    const [type, setType] = useState<TransactionType>(TRANSACTION_TYPE_EXPENSE);
    const [amountText, setAmountText] = useState('');
    const [categoryId, setCategoryId] = useState<string | null>(null);
    const [accountId, setAccountId] = useState<string | null>(null);
    const [comment, setComment] = useState('');
    const [time, setTime] = useState(Date.now());
    const [loading, setLoading] = useState(true);
    const [recognitionNote, setRecognitionNote] = useState<string | null>(null);

    useEffect(() => {
        let cancelled = false;

        (async () => {
            const [loadedCategories, loadedAccounts] = await Promise.all([listCategories(), listAccounts()]);

            if (cancelled) {
                return;
            }

            setCategories(loadedCategories);
            setAccounts(loadedAccounts);

            let nextAccountId = session?.defaultAccountId ?? null;

            // Prefill from a recognised receipt. Anything the model could not
            // determine is deliberately left blank rather than guessed at, so
            // the user can see what still needs deciding.
            if (photoId) {
                const photo = await getPhoto(photoId);

                if (photo?.recognized) {
                    const recognized = photo.recognized;

                    if (recognized.type) {
                        setType(recognized.type);
                    }

                    if (recognized.sourceAmount !== undefined) {
                        setAmountText(formatMinorUnits(Math.abs(recognized.sourceAmount)));
                    }

                    if (recognized.categoryId) {
                        setCategoryId(recognized.categoryId);
                    }

                    if (recognized.sourceAccountId) {
                        nextAccountId = recognized.sourceAccountId;
                    }

                    if (recognized.comment) {
                        setComment(recognized.comment);
                    }

                    if (recognized.time) {
                        setTime(recognized.time);
                    }
                } else if (photo?.lastError) {
                    setRecognitionNote(`This receipt could not be read (${photo.lastError}). Enter it by hand.`);
                }
            }

            if (!cancelled) {
                setAccountId(nextAccountId && nextAccountId !== '0' ? nextAccountId : loadedAccounts[0]?.id ?? null);
                setLoading(false);
            }
        })().catch(() => {
            if (!cancelled) {
                setLoading(false);
            }
        });

        return () => {
            cancelled = true;
        };
    }, [photoId, session?.defaultAccountId]);

    const visibleCategories = useMemo(() => {
        const wanted = type === TRANSACTION_TYPE_INCOME ? CATEGORY_TYPE_INCOME : CATEGORY_TYPE_EXPENSE;
        return categories.filter((category) => category.type === wanted);
    }, [categories, type]);

    const amount = parseAmountToMinorUnits(amountText);
    const canSave = amount !== null && amount !== 0 && categoryId && accountId && !loading;

    async function handleSave(): Promise<void> {
        if (amount === null || !categoryId || !accountId) {
            return;
        }

        try {
            await insertTransaction({
                type,
                categoryId,
                sourceAccountId: accountId,
                destinationAccountId: '0',
                sourceAmount: Math.abs(amount),
                destinationAmount: 0,
                time,
                utcOffset: -new Date(time).getTimezoneOffset(),
                comment: comment.trim(),
                tagIds: [],
                photoId: photoId ?? null
            });

            // The corrected row replaces the rejected one, so the bad version is
            // not left behind to be retried or counted twice.
            if (retryTransactionId) {
                await deleteTransaction(retryTransactionId);
            }

            if (photoId) {
                await markPhotoState(photoId, 'resolved');
            }

            notifyDataChanged();
            navigation.goBack();
        } catch (error) {
            Alert.alert('Could not save', error instanceof Error ? error.message : String(error));
        }
    }

    return (
        <KeyboardAvoidingView style={styles.screen} behavior={Platform.OS === 'ios' ? 'padding' : undefined}>
            <ScrollView contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled">
                {recognitionNote ? (
                    <View style={[styles.card, { borderColor: colors.warning }]}>
                        <Text style={styles.body}>{recognitionNote}</Text>
                    </View>
                ) : null}

                <View style={styles.card}>
                    <Text style={styles.label}>Type</Text>
                    <View style={styles.row}>
                        <Chip
                            label="Expense"
                            selected={type === TRANSACTION_TYPE_EXPENSE}
                            onPress={() => {
                                setType(TRANSACTION_TYPE_EXPENSE);
                                setCategoryId(null);
                            }}
                        />
                        <Chip
                            label="Income"
                            selected={type === TRANSACTION_TYPE_INCOME}
                            onPress={() => {
                                setType(TRANSACTION_TYPE_INCOME);
                                setCategoryId(null);
                            }}
                        />
                    </View>

                    <Text style={styles.label}>Amount</Text>
                    <TextInput
                        style={[styles.input, { fontSize: 24, fontWeight: '700' }]}
                        value={amountText}
                        onChangeText={setAmountText}
                        placeholder="0.00"
                        keyboardType="decimal-pad"
                        inputMode="decimal"
                        autoFocus={!photoId}
                    />
                    {amountText && amount === null ? (
                        <Text style={styles.errorText}>Enter an amount like 12.34</Text>
                    ) : null}

                    <Text style={styles.label}>Note</Text>
                    <TextInput
                        style={styles.input}
                        value={comment}
                        onChangeText={setComment}
                        placeholder="What was it?"
                        maxLength={255}
                    />
                </View>

                <View style={styles.card}>
                    <Text style={styles.label}>Category</Text>
                    <View style={[styles.row, { flexWrap: 'wrap' }]}>
                        {visibleCategories.length ? (
                            visibleCategories.map((category) => (
                                <Chip
                                    key={category.id}
                                    label={category.name}
                                    selected={categoryId === category.id}
                                    onPress={() => setCategoryId(category.id)}
                                />
                            ))
                        ) : (
                            <Text style={styles.subtitle}>
                                No categories yet — press Upload on the home screen to fetch them.
                            </Text>
                        )}
                    </View>
                </View>

                <View style={styles.card}>
                    <Text style={styles.label}>Account</Text>
                    <View style={[styles.row, { flexWrap: 'wrap' }]}>
                        {accounts.map((account) => (
                            <Chip
                                key={account.id}
                                label={account.name}
                                selected={accountId === account.id}
                                onPress={() => setAccountId(account.id)}
                            />
                        ))}
                    </View>
                </View>

                <TouchableOpacity
                    style={[styles.button, !canSave && styles.buttonDisabled]}
                    onPress={() => void handleSave()}
                    disabled={!canSave}
                >
                    <Text style={styles.buttonText}>Save locally</Text>
                </TouchableOpacity>
                <Text style={[styles.subtitle, { textAlign: 'center' }]}>
                    Stays on your phone until you press Upload.
                </Text>
            </ScrollView>
        </KeyboardAvoidingView>
    );
}

function Chip({
    label,
    selected,
    onPress
}: {
    label: string;
    selected: boolean;
    onPress: () => void;
}): React.ReactElement {
    return (
        <TouchableOpacity
            onPress={onPress}
            style={{
                paddingVertical: spacing.sm,
                paddingHorizontal: spacing.md,
                borderRadius: 999,
                borderWidth: 1,
                borderColor: selected ? colors.primary : colors.border,
                backgroundColor: selected ? colors.primary : colors.surface,
                marginBottom: spacing.xs
            }}
        >
            <Text style={{ color: selected ? colors.primaryText : colors.text, fontSize: 14 }}>{label}</Text>
        </TouchableOpacity>
    );
}
