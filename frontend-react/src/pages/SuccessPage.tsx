import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { authApi } from "../api/client";
import AuthCard from "../components/AuthCard";
import Button from "../components/Button";

export default function SuccessPage() {
    const navigate = useNavigate();
    const [email, setEmail] = useState<string | null>(null);

    useEffect(() => {
        let cancelled = false; // guards against React StrictMode's double effect run
        authApi
            .me()
            .then((user) => {
                if (!cancelled) setEmail(user.email);
            })
            .catch(() => {
                if (!cancelled) navigate("/login", { replace: true });
            });
        return () => {
            cancelled = true;
        };
    }, [navigate]);

    async function handleLogout() {
        await authApi.logout().catch(() => { }); // leave even if the request fails
        navigate("/login", { replace: true });
    }

    if (!email) {
        return (
            <AuthCard title="Checking your session...">
                <p className="text-sm text-slate-500">One moment.</p>
            </AuthCard>
        );
    }

    return (
        <AuthCard title="Login successful" subtitle="Both your password and your authenticator code were verified.">
            <div className="mb-6 rounded-lg bg-emerald-50 px-4 py-3 text-sm text-emerald-800 ring-1 ring-emerald-200">
                Signed in as <span className="font-semibold">{email}</span>
            </div>
            <Button onClick={handleLogout}>Log out</Button>
        </AuthCard>
    );
}