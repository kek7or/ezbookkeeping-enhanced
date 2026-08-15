import React, { useState } from 'react';
import {
    ActivityIndicator,
    KeyboardAvoidingView,
    Platform,
    ScrollView,
    Text,
    TextInput,
    TouchableOpacity,
    View
} from 'react-native';

import { ApiClient, ApiError } from '../api/client';
import { useApp } from '../state/AppContext';
import { normaliseServerUrl } from '../session/session';
import { colors, styles } from '../ui/theme';

/** One year. The token is what keeps the app usable without storing a password. */
const API_TOKEN_LIFETIME_SECONDS = 365 * 24 * 60 * 60;

export function LoginScreen(): React.ReactElement {
    const { signIn } = useApp();
    const [serverUrl, setServerUrl] = useState('');
    const [username, setUsername] = useState('');
    const [password, setPassword] = useState('');
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState<string | null>(null);

    const canSubmit = serverUrl.trim() && username.trim() && password && !busy;

    async function handleConnect(): Promise<void> {
        setBusy(true);
        setError(null);

        try {
            const url = normaliseServerUrl(serverUrl);
            const client = new ApiClient({ serverUrl: url, token: null });
            const auth = await client.login(username.trim(), password);

            if (auth.need2FA) {
                // Two-factor would need its own passcode step. Rather than
                // half-supporting it, say so plainly.
                setError('This account uses two-factor authentication, which this app does not support yet.');
                return;
            }

            // Swap the session token for a long-lived API token so the app keeps
            // working without the password. If the server has API tokens
            // disabled, fall back to the session token and carry on.
            client.setCredentials({ serverUrl: url, token: auth.token });
            let token = auth.token;

            try {
                const apiToken = await client.generateApiToken(password, API_TOKEN_LIFETIME_SECONDS);
                token = apiToken.token;
            } catch (tokenError) {
                if (!(tokenError instanceof ApiError)) {
                    throw tokenError;
                }
            }

            await signIn({
                serverUrl: url,
                token,
                username: auth.user.username,
                defaultCurrency: auth.user.defaultCurrency,
                defaultAccountId: auth.user.defaultAccountId
            });
        } catch (e) {
            setError(e instanceof Error ? e.message : String(e));
        } finally {
            setBusy(false);
        }
    }

    return (
        <KeyboardAvoidingView
            style={styles.screen}
            behavior={Platform.OS === 'ios' ? 'padding' : undefined}
        >
            <ScrollView contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled">
                <View style={{ gap: 4, marginBottom: 8 }}>
                    <Text style={styles.title}>Connect to your server</Text>
                    <Text style={styles.subtitle}>
                        Point the app at your ezbookkeeping instance. It stays connected afterwards.
                    </Text>
                </View>

                <View style={styles.card}>
                    <Text style={styles.label}>Server address</Text>
                    <TextInput
                        style={styles.input}
                        value={serverUrl}
                        onChangeText={setServerUrl}
                        placeholder="192.168.1.10:8080"
                        autoCapitalize="none"
                        autoCorrect={false}
                        keyboardType="url"
                        inputMode="url"
                        editable={!busy}
                    />

                    <Text style={styles.label}>Username or email</Text>
                    <TextInput
                        style={styles.input}
                        value={username}
                        onChangeText={setUsername}
                        autoCapitalize="none"
                        autoCorrect={false}
                        editable={!busy}
                    />

                    <Text style={styles.label}>Password</Text>
                    <TextInput
                        style={styles.input}
                        value={password}
                        onChangeText={setPassword}
                        secureTextEntry
                        autoCapitalize="none"
                        editable={!busy}
                        onSubmitEditing={() => {
                            if (canSubmit) {
                                void handleConnect();
                            }
                        }}
                    />
                </View>

                {error ? <Text style={styles.errorText}>{error}</Text> : null}

                <TouchableOpacity
                    style={[styles.button, !canSubmit && styles.buttonDisabled]}
                    onPress={() => void handleConnect()}
                    disabled={!canSubmit}
                >
                    {busy ? (
                        <ActivityIndicator color={colors.primaryText} />
                    ) : (
                        <Text style={styles.buttonText}>Connect</Text>
                    )}
                </TouchableOpacity>
            </ScrollView>
        </KeyboardAvoidingView>
    );
}
