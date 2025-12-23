import { cleanup } from '@testing-library/preact';
import { afterEach, beforeAll, afterAll, vi } from 'vitest';
import '@testing-library/jest-dom/vitest';
import { MockEventSource } from './mocks/sse';

// Cleanup after each test
afterEach(() => {
  cleanup();
  vi.clearAllMocks();
});

// Mock EventSource globally
beforeAll(() => {
  // @ts-expect-error - Replacing browser EventSource with mock
  global.EventSource = MockEventSource;
});

afterAll(() => {
  vi.restoreAllMocks();
});

// Mock fetch globally for tests that don't use MSW
global.fetch = vi.fn();

// Mock scrollIntoView (not implemented in JSDOM)
Element.prototype.scrollIntoView = vi.fn();

// Silence console.error in tests unless debugging
const originalError = console.error;
beforeAll(() => {
  console.error = (...args: unknown[]) => {
    // Ignore expected React/Preact warnings
    const message = args[0];
    if (
      typeof message === 'string' &&
      (message.includes('Warning:') || message.includes('act(...)'))
    ) {
      return;
    }
    originalError.apply(console, args);
  };
});

afterAll(() => {
  console.error = originalError;
});
