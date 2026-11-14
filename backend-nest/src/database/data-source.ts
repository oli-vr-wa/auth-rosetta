import { config } from 'dotenv';
import { DataSource } from 'typeorm';
import { buildDataSourceOptions } from './database.config';

config({ path: '../.env' });

export default new DataSource(buildDataSourceOptions((key) => process.env[key]));