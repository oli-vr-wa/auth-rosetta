import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { ConfigService } from '@nestjs/config';
import { ValidationPipe } from '@nestjs/common';

async function bootstrap() {
    const app = await NestFactory.create(AppModule);
    app.useGlobalPipes(new ValidationPipe({ whitelist: true, transform: true }));

    const config = app.get(ConfigService);
    const port = Number(config.get<string>('PORT') || 5002);

    await app.listen(port);
    console.log(`NestJS API is running on: http://localhost:${port}`);
}
void bootstrap();
