import { useState, type FormEvent } from "react";
import { Link } from "react-router-dom";
import { QRCodeSVG } from "qrcode.react";
import { authApi } from "../api/client";
import { errorMessage } from "../api/http";
import type { RegisterResponse } from "../api/types";
import AuthCard from "../components/AuthCard";
import Field from "../components/Field";
import Button from "../components/Button";
import ErrorMessage from "../components/ErrorMessage";

type Step = "form" | "confirm" | "done";

export default function RegisterPage() {
    const [step, setStep] = useState<Step>("form");
    const [email, setEmail] = useState("");
    const [password, setPassword] = useState("");
    const [code, setCode] = useState("");
    const [setup, setSetup] = useState<RegisterResponse | null>(null);
    const [error, setError] = useState<string | null>(null);
    const [loading, setLoading] = useState(false);

    const handleRegister = async (e: FormEvent) => {
        e.preventDefault();
        setError(null);
        setLoading(true);
        try {
            setSetup(await authApi.register(email, password));
            setPassword("");  // Do not need to keep password in memory after registration
            setStep("confirm");
        } catch (err) {
            setError(errorMessage(err));
        } finally {
            setLoading(false);
        }
    }

    const handleConfirm = async (e: FormEvent) => {
        e.preventDefault();
        setError(null);
        setLoading(true);
        try {
            await authApi.confirm2fa(email, code);
            setStep("done");
        } catch (err) {
            setError(errorMessage(err));
        } finally {
            setLoading(false);
        }
    }

    if (step === "done") {
        return (
            <AuthCard
                title="You're all set!"
                subtitle="Two-factor authentication is enabled.">

                <Link to="/login" className="block w-full rounded-lg bg-indigo-600 px-4 py-2.5 text-center font-medium text-white shadow-sm hover:bg-indigo-500">
                    Continue to login
                </Link>
            </AuthCard>
        );
    }

    if (step === "confirm" && setup) {
        return (
            <AuthCard
                title="Set up two-factor authentication"
                subtitle="Scan the QR code with an authenticator app, then enter the 6-digit code.">

                <form onSubmit={handleConfirm} className="space-y-4">
                    <div className="flex justify-center">
                        <div className="rounded-lg bg-white p-3 ring-1 ring-slate-200">
                            <QRCodeSVG value={setup.otpauthUri} size={176} />
                        </div>
                    </div>

                    <div className="text-center text-sm text-slate-500">
                        Can't scan? Enter this key manually:
                        <code className="mt-1 block select-all break-all rounded bg-slate-100 px-2 py-1 font-mono text-slate-800">
                            {setup.secret}
                        </code>
                    </div>

                    <Field
                        label="6-digit code"
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
                        {loading ? "Verifying..." : "Confirm and enable 2FA"}
                    </Button>
                </form>

            </AuthCard>
        );
    }

    return (
        <AuthCard
            title="Create an account"
            subtitle="You'll set up two-factor authentication next.">

            <form onSubmit={handleRegister} className="space-y-4">
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
                    autoComplete="new-password"
                    minLength={8}
                    required
                    disabled={loading}
                />
                <p className="text-xs text-slate-500">At least 8 characters.</p>

                <ErrorMessage message={error} />
                <Button type="submit" disabled={loading}>
                    {loading ? "Creating account..." : "Register"}
                </Button>
            </form>

            <p className="mt-6 text-center text-sm text-slate-500">
                Already registered?{" "}
                <Link to="/login" className="font-medium text-indigo-600 hover:text-indigo-500">
                    Log in
                </Link>
            </p>
        </AuthCard>
    );
}