import { request } from "./http";
import type { MeResponse, MessageResponse, RegisterResponse } from "./types";

const AUTH_API = "/api/auth"; 

export const authApi = {
    register: (email: string, password: string) =>
        request<RegisterResponse>(`${AUTH_API}/register`, { method: 'POST', body: { email, password } }),

    confirm2fa: (email: string, code: string) => 
        request<MessageResponse>(`${AUTH_API}/confirm-2fa`, { method: 'POST', body: { email, code } }),

    login: (email: string, password: string, code: string) =>
        request<MessageResponse>(`${AUTH_API}/login`, { method: 'POST', body: { email, password, code } }),

    me: () => request<MeResponse>(`${AUTH_API}/me`),

    logout: () => request<MessageResponse>(`${AUTH_API}/logout`, { method: 'POST' }),
}

