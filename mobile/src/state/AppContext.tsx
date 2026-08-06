import React, { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react';

import { ApiClient } from '../api/client';
import { openDatabase } from '../db/schema';
import { clearSession, loadSession, saveSession } from '../session/session';

import type { Session } from '../session/session';

interface AppState {
    /** Null while the stored session and database are still being opened. */
    ready: boolean;
    session: Session | null;
    client: ApiClient | null;
    signIn: (session: Session) => Promise<void>;
    signOut: () => Promise<void>;
    /** Bumped whenever local data changes, so screens can re-read from SQLite. */
    dataVersion: number;
    notifyDataChanged: () => void;
}

const AppContext = createContext<AppState | null>(null);

export function AppProvider({ children }: { children: React.ReactNode }): React.ReactElement {
    const [ready, setReady] = useState(false);
    const [session, setSession] = useState<Session | null>(null);
    const [dataVersion, setDataVersion] = useState(0);

    useEffect(() => {
        let cancelled = false;

        (async () => {
            // Opening the database here runs migrations once, up front, so no
            // screen has to cope with an unmigrated schema.
            await openDatabase();
            const stored = await loadSession();

            if (!cancelled) {
                setSession(stored);
                setReady(true);
            }
        })().catch(() => {
            if (!cancelled) {
                setReady(true);
            }
        });

        return () => {
            cancelled = true;
        };
    }, []);

    const client = useMemo(() => {
        if (!session) {
            return null;
        }

        return new ApiClient({ serverUrl: session.serverUrl, token: session.token });
    }, [session]);

    const signIn = useCallback(async (next: Session) => {
        await saveSession(next);
        setSession(next);
    }, []);

    const signOut = useCallback(async () => {
        // Local rows are deliberately left in place: signing out must never be
        // a way to silently lose transactions that were never uploaded.
        await clearSession();
        setSession(null);
    }, []);

    const notifyDataChanged = useCallback(() => {
        setDataVersion((version) => version + 1);
    }, []);

    const value = useMemo<AppState>(
        () => ({ ready, session, client, signIn, signOut, dataVersion, notifyDataChanged }),
        [ready, session, client, signIn, signOut, dataVersion, notifyDataChanged]
    );

    return <AppContext.Provider value={value}>{children}</AppContext.Provider>;
}

export function useApp(): AppState {
    const context = useContext(AppContext);

    if (!context) {
        throw new Error('useApp must be used inside an AppProvider');
    }

    return context;
}
