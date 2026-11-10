import type { InputHTMLAttributes } from "react";

interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
    label: string;
    name: string;
}

export default function Input({ label, name, ...props }: InputProps) {
    return (
        <div>
            <label htmlFor={name} className="block text-sm font-medium text-slate-700">
                {label}
            </label>
            <input
                id={name}
                name={name}
                className="mt-1 block w-full rounded-lg border border-slate-300 px-3 py-2 text-slate-900 shadow-sm focus:border-indigo-500 focus:outline-none focus:ring-2 focus:ring-indigo-200 disabled:bg-slate-100"
                {...props}
            />
        </div>
    );
}