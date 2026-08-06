import type {
    AccountInfoResponse,
    ApiResponse,
    AuthResponse,
    ReceiptJobInfoResponse,
    ReceiptJobSubmitResponse,
    TokenGenerateApiResponse,
    TransactionCategoryInfoResponse,
    TransactionImportRequest,
    TransactionPictureInfoResponse,
    TransactionTagInfoResponse
} from './types';

/** Thrown when the server answered but reported a failure. */
export class ApiError extends Error {
    public readonly errorCode: number;
    public readonly httpStatus: number;

    public constructor(message: string, errorCode: number, httpStatus: number) {
        super(message);
        this.name = 'ApiError';
        this.errorCode = errorCode;
        this.httpStatus = httpStatus;
    }

    /**
     * Whether retrying this exact request later could plausibly succeed.
     * Client-side validation failures never will, so the sync worker parks
     * those rows for the user instead of burning attempts on them.
     */
    public get isRetryable(): boolean {
        return this.httpStatus >= 500 || this.httpStatus === 429;
    }

    /** Whether our credentials are the problem, so syncing should stop entirely. */
    public get isAuthFailure(): boolean {
        return this.httpStatus === 401;
    }
}

/** Thrown when the request never reached the server (offline, DNS, TLS, timeout). */
export class NetworkError extends Error {
    public constructor(message: string) {
        super(message);
        this.name = 'NetworkError';
    }
}

export interface ApiCredentials {
    readonly serverUrl: string;
    readonly token: string | null;
}

const REQUEST_TIMEOUT_MS = 30_000;
/**
 * Image uploads are bounded by bandwidth rather than server work — recognition
 * itself is queued server-side and never held open on a request.
 */
const UPLOAD_TIMEOUT_MS = 120_000;

function joinUrl(serverUrl: string, path: string): string {
    return `${serverUrl.replace(/\/+$/, '')}${path}`;
}

/**
 * The server derives transaction timestamps from these when the client does not
 * supply an explicit offset, and rejects requests it cannot resolve a timezone
 * for. Sending both lets it prefer the IANA name and fall back to the offset.
 */
function timezoneHeaders(): Record<string, string> {
    return {
        'X-Timezone-Offset': String(-new Date().getTimezoneOffset()),
        'X-Timezone-Name': Intl.DateTimeFormat().resolvedOptions().timeZone
    };
}

export class ApiClient {
    private credentials: ApiCredentials;

    public constructor(credentials: ApiCredentials) {
        this.credentials = credentials;
    }

    public setCredentials(credentials: ApiCredentials): void {
        this.credentials = credentials;
    }

    public get serverUrl(): string {
        return this.credentials.serverUrl;
    }

    private async request<T>(
        method: 'GET' | 'POST',
        path: string,
        options: { body?: unknown; form?: FormData; timeoutMs?: number } = {}
    ): Promise<T> {
        const headers: Record<string, string> = { ...timezoneHeaders() };

        if (this.credentials.token) {
            headers['Authorization'] = `bearer ${this.credentials.token}`;
        }

        let body: string | FormData | undefined;

        if (options.form) {
            // Deliberately no Content-Type: fetch must set the multipart boundary.
            body = options.form;
        } else if (options.body !== undefined) {
            headers['Content-Type'] = 'application/json';
            body = JSON.stringify(options.body);
        }

        const controller = new AbortController();
        const timeout = setTimeout(() => controller.abort(), options.timeoutMs ?? REQUEST_TIMEOUT_MS);
        let response: Response;

        try {
            response = await fetch(joinUrl(this.credentials.serverUrl, path), {
                method,
                headers,
                body,
                signal: controller.signal
            });
        } catch (e) {
            const reason = controller.signal.aborted ? 'request timed out' : (e as Error).message;
            throw new NetworkError(reason);
        } finally {
            clearTimeout(timeout);
        }

        let payload: ApiResponse<T>;

        try {
            payload = (await response.json()) as ApiResponse<T>;
        } catch {
            // A non-JSON body means we are almost certainly not talking to an
            // ezbookkeeping server — a captive portal or a stray reverse proxy.
            throw new ApiError(
                `server returned ${response.status} with a non-JSON body`,
                0,
                response.status
            );
        }

        if (!payload.success) {
            throw new ApiError(payload.errorMessage, payload.errorCode, response.status);
        }

        return payload.result;
    }

    // --- auth ---

    public login(username: string, password: string): Promise<AuthResponse> {
        return this.request<AuthResponse>('POST', '/api/authorize.json', {
            body: { loginName: username, password }
        });
    }

    /**
     * Exchanges the short-lived session token for a long-lived API token, so the
     * app does not have to hold the password or re-login on every trip out.
     * Requires `enable_api_token` on the server.
     */
    public generateApiToken(password: string, expiredInSeconds: number): Promise<TokenGenerateApiResponse> {
        return this.request<TokenGenerateApiResponse>('POST', '/api/v1/tokens/generate/api.json', {
            body: { password, expiredInSeconds }
        });
    }

    // --- pull ---

    public listCategories(): Promise<TransactionCategoryInfoResponse[]> {
        return this.request<TransactionCategoryInfoResponse[]>('GET', '/api/v1/transaction/categories/list.json');
    }

    public listAccounts(): Promise<AccountInfoResponse[]> {
        return this.request<AccountInfoResponse[]>('GET', '/api/v1/accounts/list.json');
    }

    public listTags(): Promise<TransactionTagInfoResponse[]> {
        return this.request<TransactionTagInfoResponse[]>('GET', '/api/v1/transaction/tags/list.json');
    }

    // --- push ---

    /**
     * Bulk-creates transactions. Resending the same clientSessionId after a
     * dropped connection returns the original count rather than double-importing
     * (pkg/api/transactions.go TransactionImportHandler), which is what makes the
     * outbox safe to retry.
     */
    public importTransactions(request: TransactionImportRequest): Promise<number> {
        return this.request<number>('POST', '/api/v1/transactions/import.json', { body: request });
    }

    /** Returns import progress as 0-100, or null if the server has no record of the session. */
    public getImportProgress(clientSessionId: string): Promise<number | null> {
        return this.request<number | null>(
            'GET',
            `/api/v1/transactions/import/process.json?client_session_id=${encodeURIComponent(clientSessionId)}`
        );
    }

    public uploadPicture(fileUri: string, fileName: string): Promise<TransactionPictureInfoResponse> {
        const form = new FormData();
        form.append('picture', {
            uri: fileUri,
            name: fileName,
            type: 'image/jpeg'
        } as unknown as Blob);

        return this.request<TransactionPictureInfoResponse>('POST', '/api/v1/transaction/pictures/upload.json', {
            form,
            timeoutMs: UPLOAD_TIMEOUT_MS
        });
    }

    /**
     * Queues a receipt for recognition. Returns as soon as the server has stored
     * the image — the model round-trip happens later, on the server's own time.
     * This is why the app never blocks on recognition.
     */
    public submitReceiptJob(fileUri: string, fileName: string): Promise<ReceiptJobSubmitResponse> {
        const form = new FormData();
        form.append('image', {
            uri: fileUri,
            name: fileName,
            type: 'image/jpeg'
        } as unknown as Blob);

        return this.request<ReceiptJobSubmitResponse>('POST', '/api/v1/receipt/jobs/submit.json', {
            form,
            timeoutMs: UPLOAD_TIMEOUT_MS
        });
    }

    /** Returns the queue's current state: what is still working, and what is ready to review. */
    public listReceiptJobs(): Promise<ReceiptJobInfoResponse[]> {
        return this.request<ReceiptJobInfoResponse[]>('GET', '/api/v1/receipt/jobs/list.json');
    }

    /** Closes a job once the user has created a transaction from it or discarded it. */
    public resolveReceiptJob(jobId: string): Promise<boolean> {
        return this.request<boolean>('POST', '/api/v1/receipt/jobs/resolve.json', { body: { id: jobId } });
    }
}
