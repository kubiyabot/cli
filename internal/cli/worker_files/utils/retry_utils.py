"""Retry utilities with exponential backoff"""

import time
import httpx
from typing import Callable, TypeVar, Optional
from functools import wraps
import structlog

logger = structlog.get_logger()

T = TypeVar('T')


def retry_with_backoff(
    max_retries: int = 3,
    initial_delay: float = 1.0,
    backoff_factor: float = 2.0,
    exceptions: tuple = (httpx.TimeoutException, httpx.ConnectError)
):
    """
    Decorator for retrying functions with exponential backoff

    Args:
        max_retries: Maximum number of retry attempts
        initial_delay: Initial delay in seconds
        backoff_factor: Multiplier for each retry (exponential)
        exceptions: Tuple of exceptions to catch and retry
    """
    def decorator(func: Callable[..., T]) -> Callable[..., T]:
        @wraps(func)
        def wrapper(*args, **kwargs) -> T:
            last_exception = None

            for attempt in range(max_retries):
                try:
                    return func(*args, **kwargs)
                except exceptions as e:
                    last_exception = e
                    if attempt < max_retries - 1:
                        delay = initial_delay * (backoff_factor ** attempt)
                        logger.warning(
                            f"{func.__name__} failed, retrying...",
                            extra={
                                "attempt": attempt + 1,
                                "max_retries": max_retries,
                                "delay": delay,
                                "error": str(e)
                            }
                        )
                        time.sleep(delay)
                    else:
                        logger.error(
                            f"{func.__name__} failed after {max_retries} attempts",
                            extra={"error": str(e)}
                        )

            raise last_exception

        return wrapper
    return decorator
