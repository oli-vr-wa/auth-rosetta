import type { ReactNode } from "react";

interface AuthCardProps {
    title: string;
    subtitle?: string;
    children: ReactNode;
}

export default function AuthCard({ title, subtitle, children }: AuthCardProps) {
    return (
        <main className="flex min-h-screen items-center justify-center bg-slate-50 px-4">
            <div className="w-full max-w-md rounded-2xl bg-white p-8 shadow-lg ring-1 ring-slate-200">
                <h1 className="text-2xl font-bold text-slate-900">{title}</h1>
                {subtitle && <p className="mt-1 font-semibold text-slate-500">{subtitle}</p>}
                <div className="mt-6">{children}</div>
            </div>
        </main>
    );
}