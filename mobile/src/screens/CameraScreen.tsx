import React, { useRef, useState } from 'react';
import { ActivityIndicator, Alert, Text, TouchableOpacity, View } from 'react-native';
import { CameraView, useCameraPermissions } from 'expo-camera';
import { Directory, File, Paths } from 'expo-file-system';

import { insertPhoto } from '../db/repo';
import { useApp } from '../state/AppContext';
import { colors, spacing, styles } from '../ui/theme';

import type { ScreenProps } from '../navigation/types';

/**
 * Receipts are copied out of the cache into the document directory. The cache
 * is reclaimable by Android at any time, and a receipt the user has not
 * uploaded yet must not be one low-storage moment away from disappearing.
 */
const RECEIPTS_DIRECTORY = 'receipts';

function receiptsDirectory(): Directory {
    const directory = new Directory(Paths.document, RECEIPTS_DIRECTORY);

    if (!directory.exists) {
        directory.create({ intermediates: true });
    }

    return directory;
}

export function CameraScreen({ navigation }: ScreenProps<'Camera'>): React.ReactElement {
    const { notifyDataChanged } = useApp();
    const [permission, requestPermission] = useCameraPermissions();
    const [busy, setBusy] = useState(false);
    const [captured, setCaptured] = useState(0);
    const cameraRef = useRef<CameraView>(null);

    if (!permission) {
        return (
            <View style={[styles.screen, { justifyContent: 'center', alignItems: 'center' }]}>
                <ActivityIndicator color={colors.primary} />
            </View>
        );
    }

    if (!permission.granted) {
        return (
            <View style={[styles.screen, styles.content, { justifyContent: 'center' }]}>
                <Text style={styles.title}>Camera access needed</Text>
                <Text style={styles.subtitle}>
                    The app needs the camera to photograph receipts. Photos stay on your phone until you upload them.
                </Text>
                <TouchableOpacity style={styles.button} onPress={() => void requestPermission()}>
                    <Text style={styles.buttonText}>Allow camera</Text>
                </TouchableOpacity>
            </View>
        );
    }

    async function handleCapture(): Promise<void> {
        if (!cameraRef.current || busy) {
            return;
        }

        setBusy(true);

        try {
            const photo = await cameraRef.current.takePictureAsync({ quality: 0.7 });

            if (!photo) {
                return;
            }

            const fileName = `receipt-${Date.now()}.jpg`;
            const source = new File(photo.uri);
            const destination = new File(receiptsDirectory(), fileName);
            source.move(destination);

            await insertPhoto(destination.uri, fileName);
            setCaptured((count) => count + 1);
            notifyDataChanged();
        } catch (error) {
            Alert.alert('Could not save the photo', error instanceof Error ? error.message : String(error));
        } finally {
            setBusy(false);
        }
    }

    return (
        <View style={{ flex: 1, backgroundColor: '#000' }}>
            <CameraView ref={cameraRef} style={{ flex: 1 }} facing="back" />

            <View style={{ padding: spacing.lg, gap: spacing.md, backgroundColor: colors.surface }}>
                <Text style={[styles.subtitle, { textAlign: 'center' }]}>
                    {captured
                        ? `${captured} receipt${captured === 1 ? '' : 's'} saved. Keep going, or go back.`
                        : 'Fit the whole receipt in the frame.'}
                </Text>

                <TouchableOpacity
                    style={[styles.button, busy && styles.buttonDisabled]}
                    onPress={() => void handleCapture()}
                    disabled={busy}
                >
                    {busy ? (
                        <ActivityIndicator color={colors.primaryText} />
                    ) : (
                        <Text style={styles.buttonText}>Capture</Text>
                    )}
                </TouchableOpacity>

                <TouchableOpacity
                    style={[styles.button, styles.buttonSecondary]}
                    onPress={() => navigation.goBack()}
                >
                    <Text style={[styles.buttonText, styles.buttonSecondaryText]}>Done</Text>
                </TouchableOpacity>
            </View>
        </View>
    );
}
