import { useState, type FormEvent } from "react";
import { Link, useNavigate } from "react-router-dom";
import { authApi } from "../api/client";
import { errorMessage } from "../api/http";
import AuthCard from "../components/AuthCard";
import Field from "../components/Field";
import Button from "../components/Button";
import ErrorMessage from "../components/ErrorMessage";

export default function LoginPage() {
    const navigate = useNavigate();
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");
    const [code, setCode] = useState("");
    const [error, setError] = useState<string | null>(null);
    const [loading, setLoading] = useState(false);

    async function handleSubmit(e: FormEvent) {
        e.preventDefault();
        setError(null);
        setLoading(true);
        try {
            await authApi.login(email, password, code); // server sets the cookie
            navigate("/success", { replace: true });
        } catch (err) {
            setError(errorMessage(err));
            setCode(""); // codes are single-use in practice, so force a fresh one
        } finally {
            setLoading(false);
        }
    }

    return (
        <AuthCard title="Welcome back" subtitle="Log in with your password and authenticator code.">
            <form onSubmit={handleSubmit} className="space-y-4">
                <Field
                    label="Email"
                    name="email"
                    type="email"
                    value={email}
                    onChange={(e) => setEmail(e.target.value)}
                    autoComplete="email"
                    required
                    disabled={loading}
                />
                <Field
                    label="Password"
                    name="password"
                    type="password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                    autoComplete="current-password"
                    required
                    disabled={loading}
                />
                <Field
                    label="Authenticator code"
                    name="code"
                    value={code}
                    onChange={(e) => setCode(e.target.value.replace(/\D/g, ""))}
                    inputMode="numeric"
                    autoComplete="one-time-code"
                    maxLength={6}
                    pattern="[0-9]{6}"
                    placeholder="123456"
                    required
                    disabled={loading}
                />

                <ErrorMessage message={error} />
                <Button type="submit" disabled={loading || code.length !== 6}>
                    {loading ? "Logging in..." : "Log in"}
                </Button>
            </form>

            <p className="mt-6 text-center text-sm text-slate-500">
                No account yet?{" "}
                <Link to="/register" className="font-medium text-indigo-600 hover:text-indigo-500">
                    Register
                </Link>
            </p>
        </AuthCard>
    );
}