import { h, ComponentChildren } from 'preact';
import { render, RenderOptions } from '@testing-library/preact';
import userEvent from '@testing-library/user-event';
import { ToastProvider } from '../state';

// Re-export everything from testing-library
export * from '@testing-library/preact';

// Custom render that can include providers
interface CustomRenderOptions extends Omit<RenderOptions, 'wrapper'> {
  // Add any custom options here, like initial state
}

function AllTheProviders({ children }: { children: ComponentChildren }) {
  // Wrap with providers needed for testing
  return <ToastProvider>{children}</ToastProvider>;
}

function customRender(ui: preact.JSX.Element, options?: CustomRenderOptions) {
  return render(ui, { wrapper: AllTheProviders, ...options });
}

// Override render with custom render
export { customRender as render };

// Setup userEvent with custom options
export function setupUser() {
  return userEvent.setup();
}

// Helper to wait for element to be removed
export async function waitForElementToBeRemoved(
  callback: () => Element | null,
  options?: { timeout?: number }
) {
  const timeout = options?.timeout ?? 1000;
  const startTime = Date.now();

  while (callback() !== null) {
    if (Date.now() - startTime > timeout) {
      throw new Error('Timed out waiting for element to be removed');
    }
    await new Promise((resolve) => setTimeout(resolve, 50));
  }
}

// Helper to create a mock Response
export function mockResponse<T>(data: T, options?: { status?: number; ok?: boolean }) {
  return {
    ok: options?.ok ?? true,
    status: options?.status ?? 200,
    json: () => Promise.resolve(data),
    text: () => Promise.resolve(JSON.stringify(data)),
  } as Response;
}

// Helper to mock fetch with specific responses
export function mockFetchResponses(responses: Record<string, unknown>) {
  return (url: string) => {
    const path = url.replace(/^https?:\/\/[^/]+/, '').split('?')[0];
    const data = responses[path];

    if (data === undefined) {
      return Promise.resolve(mockResponse({ error: 'Not found' }, { status: 404, ok: false }));
    }

    return Promise.resolve(mockResponse(data));
  };
}

// Helper to wait for async operations
export async function waitForAsync(ms: number = 0) {
  await new Promise((resolve) => setTimeout(resolve, ms));
}
