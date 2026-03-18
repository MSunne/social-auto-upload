import asyncio
import time
from functools import wraps

from utils.log import network_logger


def async_retry(timeout=60, max_retries=None):
    def decorator(func):
        @wraps(func)
        async def wrapper(*args, **kwargs):
            start_time = time.time()
            attempts = 0
            while True:
                try:
                    return await func(*args, **kwargs)
                except Exception as e:
                    attempts += 1
                    if max_retries is not None and attempts >= max_retries:
                        network_logger.error("async retry reached max retries={} error={}", max_retries, e)
                        raise Exception(f"Failed after {max_retries} retries.") from e
                    if time.time() - start_time > timeout:
                        network_logger.error("async retry timed out timeout_seconds={} error={}", timeout, e)
                        raise TimeoutError(f"Function execution exceeded {timeout} seconds timeout.") from e
                    network_logger.warning("async retry attempt={} failed error={} retrying_in_seconds=1", attempts, e)
                    await asyncio.sleep(1)  # Sleep to avoid tight loop or provide backoff logic here

        return wrapper

    return decorator
