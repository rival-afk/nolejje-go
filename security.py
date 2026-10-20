import bcrypt
import logging

logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

def hash_password(password: str) -> str:
    password_bytes = password.encode('utf-8')[:72]
    salt = bcrypt.gensalt()
    hashed = bcrypt.hashpw(password_bytes, salt)
    logger.info(f"Password hashed successfully")
    return hashed.decode('utf-8')

def verify_password(password: str, hashed: str) -> bool:
    try:
        password_bytes = password.encode('utf-8')[:72]
        hashed_bytes = hashed.encode('utf-8')
        
        result = bcrypt.checkpw(password_bytes, hashed_bytes)
        logger.info(f"Password verification: {'success' if result else 'failed'}")
        return result
    except Exception as e:
        logger.error(f"Verification error: {e}")
        return False