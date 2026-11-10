import { NestFactory } from '@nestjs/core';
import { AppModule } from './app.module';
import { ConfigService } from '@nestjs/config';

async function bootstrap() {
    const app = await NestFactory.create(AppModule);

    const config = app.get(ConfigService);
    const port = Number(config.get<string>('PORT') || 5002);

    await app.listen(port);
    console.log(`NestJS API is running on: http://localhost:${port}`);
}
void bootstrap();
