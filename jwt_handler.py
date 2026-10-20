from jose import jwt
from datetime import datetime, timedelta
from dotenv import load_dotenv
import os

load_dotenv()

SECRET_KEY = os.getenv('jwt_secret_key')
ALGORITHM = "HS256"
TOKEN_EXPIRE = 1440 # в минутах время истечения срока действия токена

def create_access_token (data: dict):
  to_encode = data.copy()
  expire = datetime.utcnow() + timedelta(minutes=TOKEN_EXPIRE)
  to_encode.update({"exp": expire})
  encoded_jwt = jwt.encode(to_encode, SECRET_KEY, algorithm=ALGORITHM)
  return encoded_jwt