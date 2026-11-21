import { BadRequestException, INestApplication, ValidationPipe } from "@nestjs/common";
import cookieParser from "cookie-parser";
import { jsonBody, jsonBodyErrorHandler, requireJsonObject } from "./common/middleware/json-body.middleware";
import { ApiExceptionFilter } from "./common/filters/api-exception.filter";
import { ConfigService } from "@nestjs/config";


const DEFAULT_FRONTEND_ORIGIN = 'http://localhost:5173';

/**
 * Configures the NestJS application with global settings, middleware, and filters.
 * This function sets up CORS, cookie parsing, JSON body parsing, validation pipes, and exception filters for the application.
 * @param app - The NestJS application instance to configure.
 */
export function configureApp(app: INestApplication): void {
    const config = app.get(ConfigService);
    const frontendOrigin = config.get<string>('FRONTEND_ORIGIN')?.trim() || DEFAULT_FRONTEND_ORIGIN;

    // Enable CORS for the frontend origin with specific settings
    // The order matters: CORS must be enabled before any other middleware that handles requests.
    app.enableCors({
        origin: [frontendOrigin],
        credentials: true,
        methods: ['GET', 'POST', 'OPTIONS'],
        allowedHeaders: ['Content-Type'],
    });

    app.use(cookieParser());
    app.use(jsonBody, requireJsonObject, jsonBodyErrorHandler); // Middleware to parse JSON bodies and handle errors from the JSON parser

    app.useGlobalPipes(new ValidationPipe({ whitelist: true, transform: true, exceptionFactory: () => new BadRequestException('Invalid request') }));
    app.useGlobalFilters(new ApiExceptionFilter());
}