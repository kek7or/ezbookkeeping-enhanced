import * as SecureStore from 'expo-secure-store';

/**
 * Persisted connection to a server. The token is a long-lived API token rather
 * than a session token, so the app survives being closed between shopping trips
 * without holding on to the password.
 */
export interface Session {
    serverUrl: string;
    token: string;
    username: string;
    defaultCurrency: string;
    defaultAccountId: string;
}

const SESSION_KEY = 'ezb_session';

export async function loadSession(): Promise<Session | null> {
    const raw = await SecureStore.getItemAsync(SESSION_KEY);

    if (!raw) {
        return null;
    }

    try {
        return JSON.parse(raw) as Session;
    } catch {
        // A corrupt blob is unrecoverable and would wedge the app at launch,
        // so drop it and fall back to the login screen.
        await SecureStore.deleteItemAsync(SESSION_KEY);
        return null;
    }
}

export async function saveSession(session: Session): Promise<void> {
    await SecureStore.setItemAsync(SESSION_KEY, JSON.stringify(session));
}

export async function clearSession(): Promise<void> {
    await SecureStore.deleteItemAsync(SESSION_KEY);
}

/**
 * Normalises what a user actually types into a server field ("192.168.1.5:8080",
 * "example.com/ezb/") into a usable origin.
 */
export function normaliseServerUrl(input: string): string {
    const trimmed = input.trim().replace(/\/+$/, '');

    if (!trimmed) {
        return '';
    }

    if (!/^https?:\/\//i.test(trimmed)) {
        return `http://${trimmed}`;
    }

    return trimmed;
}
