const API_URL = import.meta.env.VITE_API_URL as string;

export class ApiError extends Error {
    status: number;
    constructor(status: number, message: string) {
        super(message);
        this.status = status;
    }
}

interface RequestOptions {
    method?: "GET" | "POST"; 
    body?: unknown;
}

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const { method = "GET", body } = options;

    const headers = new Headers();
    if (body !== undefined) headers.set("Content-Type", "application/json"); 

    const response = await fetch(`${API_URL}${path}`, {
        method,
        headers,
        credentials: "include",
        body: body !== undefined ? JSON.stringify(body) : undefined,
    });

    if (response.status === 204) return undefined as T;

    const data = await response.json().catch(() => null);

    if (!response.ok) throw new ApiError(response.status, data?.message || response.statusText);
    return data as T;
}

export function errorMessage(error: unknown): string {
    if (error instanceof ApiError) return error.message;
    return String(error);
}