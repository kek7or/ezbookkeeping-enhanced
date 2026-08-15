import React, { useCallback, useState } from 'react';
import { Alert, Image, ScrollView, Text, TouchableOpacity, View } from 'react-native';
import { useFocusEffect } from '@react-navigation/native';

import { formatMinorUnits } from '../utils/money';
import { listCategories, listPhotos, markPhotoState } from '../db/repo';
import { useApp } from '../state/AppContext';
import { colors, spacing, styles } from '../ui/theme';

import type { Category, Photo } from '../db/repo';
import type { ScreenProps } from '../navigation/types';

export function ReviewScreen({ navigation }: ScreenProps<'Review'>): React.ReactElement {
    const { notifyDataChanged } = useApp();
    const [photos, setPhotos] = useState<Photo[]>([]);
    const [categoriesById, setCategoriesById] = useState<Map<string, Category>>(new Map());

    const refresh = useCallback(async () => {
        const [pending, categories] = await Promise.all([listPhotos(['needs_review']), listCategories()]);

        setPhotos(pending);
        setCategoriesById(new Map(categories.map((category) => [category.id, category])));
    }, []);

    useFocusEffect(
        useCallback(() => {
            void refresh();
        }, [refresh])
    );

    function handleDiscard(photo: Photo): void {
        Alert.alert('Discard this receipt?', 'The photo stays on your phone but will not be uploaded.', [
            { text: 'Cancel', style: 'cancel' },
            {
                text: 'Discard',
                style: 'destructive',
                onPress: () => {
                    void (async () => {
                        await markPhotoState(photo.id, 'resolved');
                        notifyDataChanged();
                        await refresh();
                    })();
                }
            }
        ]);
    }

    if (!photos.length) {
        return (
            <View style={[styles.screen, styles.content, { justifyContent: 'center', alignItems: 'center' }]}>
                <Text style={styles.title}>Nothing to review</Text>
                <Text style={[styles.subtitle, { textAlign: 'center' }]}>
                    Receipts appear here once the server has finished reading them. Press Upload again to check.
                </Text>
            </View>
        );
    }

    return (
        <ScrollView style={styles.screen} contentContainerStyle={styles.content}>
            {photos.map((photo) => {
                const recognized = photo.recognized;
                const category = recognized?.categoryId ? categoriesById.get(recognized.categoryId) : undefined;

                return (
                    <View key={photo.id} style={styles.card}>
                        <Image
                            source={{ uri: photo.fileUri }}
                            style={{ width: '100%', height: 180, borderRadius: 8, backgroundColor: colors.background }}
                            resizeMode="cover"
                        />

                        {recognized ? (
                            <>
                                <Text style={styles.title}>
                                    {recognized.sourceAmount !== undefined
                                        ? formatMinorUnits(Math.abs(recognized.sourceAmount))
                                        : 'Amount not found'}
                                </Text>
                                <Text style={styles.subtitle}>
                                    {recognized.comment || 'No description read'}
                                    {category ? ` · ${category.name}` : ' · category not matched'}
                                </Text>
                            </>
                        ) : (
                            <Text style={[styles.body, { color: colors.warning }]}>
                                The server could not read this one{photo.lastError ? `: ${photo.lastError}` : ''}. You
                                can still enter it by hand.
                            </Text>
                        )}

                        <View style={[styles.row, { marginTop: spacing.sm }]}>
                            <TouchableOpacity
                                style={[styles.button, { flex: 1 }]}
                                onPress={() => navigation.navigate('AddTransaction', { photoId: photo.id })}
                            >
                                <Text style={styles.buttonText}>Check and save</Text>
                            </TouchableOpacity>
                            <TouchableOpacity
                                style={[styles.button, styles.buttonSecondary]}
                                onPress={() => handleDiscard(photo)}
                            >
                                <Text style={[styles.buttonText, styles.buttonSecondaryText]}>Discard</Text>
                            </TouchableOpacity>
                        </View>
                    </View>
                );
            })}
        </ScrollView>
    );
}
