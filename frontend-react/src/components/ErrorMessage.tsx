export default function ErrorMessage({ message }: { message: string | null }) {
    if (!message) return null;

    return (
        <p role="alert" className="rounded-lg bg-red-50 px-3 py-2 text-sm text-red-700 ring-1 ring-red-200">
            {message}
        </p>
    );
}