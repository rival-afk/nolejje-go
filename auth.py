from jose import jwt
from jose.exceptions import ExpiredSignatureError, JWTError
from fastapi import HTTPException, Depends
from fastapi.security import OAuth2PasswordBearer
from jwt_handler import SECRET_KEY, ALGORITHM
import db

auth_scheme = OAuth2PasswordBearer(tokenUrl="/auth/login")

async def get_user_func (token = Depends(auth_scheme)):
  
  try:
    decoded_token = jwt.decode(token, SECRET_KEY, algorithms=[ALGORITHM])
  
    user_id = decoded_token["user_id"]
  
    async with db.pool.acquire() as conn:
    
      rows = await conn.fetchrow("""
        SELECT * FROM users WHERE id = $1""",
        user_id)
    
      if not rows:
        raise HTTPException(
          status_code=401,
          detail="Invalid Token"
        )
  
    return rows
  except ExpiredSignatureError:
    raise HTTPException(
      status_code=401,
      detail="Expired Token"
    )
  except JWTError:
    raise HTTPException(
      status_code=401,
      detail="Invalid Token"
    )