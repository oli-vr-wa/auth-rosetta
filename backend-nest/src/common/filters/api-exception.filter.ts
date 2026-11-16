import { ArgumentsHost, Catch, ExceptionFilter, HttpException, Logger } from '@nestjs/common';
import type { Response, Request } from 'express'; 

@Catch()
export class ApiExceptionFilter implements ExceptionFilter {
    private readonly logger = new Logger(ApiExceptionFilter.name);

    /**
     * Catches exceptions thrown during request processing and sends an appropriate HTTP response.
     * @param exception - The exception that was thrown.
     * @param host - The execution context of the request.
     */
    catch(exception: unknown, host: ArgumentsHost): void {
        const http = host.switchToHttp();
        const req = http.getRequest<Request>();
        const res = http.getResponse<Response>();
        const where = `${req.method} ${req.originalUrl}`;

        // Errors thrown by NestJS and by our services
        if (exception instanceof HttpException) {
            const status = exception.getStatus();
            const message = this.messageFrom(exception);
            const line = `${where} -> ${status}: ${message}`;
            // Log the error based on its severity for debugging and monitoring purposes
            if (status >= 500) this.logger.error(line, exception.stack);
            else this.logger.warn(line);

            res.status(status).json({ error: message });
            return;
        }

        // Client errors from the framework layer
        const clientStatus = this.clientErrorStatus(exception);
        if (clientStatus !== null) {
            // Log the client error for debugging and monitoring purposes
            this.logger.warn(`${where} -> ${clientStatus}: ${this.describe(exception)}`);
            // Respond with a generic error message to the client to avoid exposing sensitive information
            res.status(clientStatus).json({ error: 'Invalid request' }); 
            return;
        }

        // Anything else is a bug: Log the details and return a generic error message to the client
        this.logger.error(
            `${where} -> 500: ${this.describe(exception)}`,
            exception instanceof Error ? exception.stack : undefined,
        );
        // Generic error message to send to the client
        res.status(500).json({ error: 'Internal server error' });
    }

    /**
     * Extracts a user-friendly error message from an HttpException.
     * @param exception - The HttpException instance from which to extract the message.
     * @returns A string representing the error message.
     */
    private messageFrom(exception: HttpException): string {
        const body = exception.getResponse();
        if (typeof body === 'string') return body;

        const message = (body as { message?: string | string[] }).message;
        if (Array.isArray(message)) return message.join(', ');
        return message ?? exception.message;
    }

    /**
     * Determines if the provided exception is a client error (HTTP status code 4xx).
     * @param exception - The exception to check.
     * @returns The HTTP status code if it's a client error, otherwise null.
     */
    private clientErrorStatus(exception: unknown): number | null {
        if (typeof exception !== 'object' || exception === null) return null;

        const status = (exception as { statusCode?: unknown }).statusCode;
        return typeof status === 'number' && status >= 400 && status < 500 ? status : null;
    }

    /**
     * Provides a string description of the exception for logging purposes.
     * @param exception - The exception to describe.
     * @returns A string representation of the exception.
     */
    private describe(exception: unknown): string {
        return exception instanceof Error ? exception.message : String(exception);
    }
}