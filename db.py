import asyncpg
from dotenv import load_dotenv
import os

load_dotenv()

db_url = os.getenv("db_url", "postgres://postgres@localhost/nolejje")

pool = None

async def create_pool():
  global pool
  pool = await asyncpg.create_pool(dsn=db_url)

async def close_pool():
  global pool
  if pool:
    await pool.close()
    pool = None