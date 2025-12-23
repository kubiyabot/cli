import { h, createContext, ComponentChildren } from 'preact';
import { useContext, useState, useCallback, useRef, useEffect } from 'preact/hooks';

// Toast types
export interface Toast {
  id: string;
  type: 'success' | 'error' | 'warning' | 'info';
  title: string;
  message?: string;
  duration?: number; // ms, 0 for persistent
  action?: {
    label: string;
    onClick: () => void;
  };
}

// Default durations by type (ms)
const DEFAULT_DURATIONS: Record<Toast['type'], number> = {
  success: 3000,
  error: 5000,
  warning: 5000,
  info: 4000,
};

// Context value interface
export interface ToastContextValue {
  toasts: Toast[];
  addToast: (toast: Omit<Toast, 'id'>) => string;
  removeToast: (id: string) => void;
  clearAllToasts: () => void;
}

// Create context
const ToastContext = createContext<ToastContextValue | undefined>(undefined);

// Provider props
interface ToastProviderProps {
  children: ComponentChildren;
  maxToasts?: number;
}

/**
 * Toast Context Provider
 * Manages toast notifications with auto-dismiss and stacking.
 */
export function ToastProvider({ children, maxToasts = 5 }: ToastProviderProps) {
  const [toasts, setToasts] = useState<Toast[]>([]);
  const timersRef = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());

  // Generate unique ID
  const generateId = useCallback(() => {
    return `toast-${Date.now()}-${Math.random().toString(36).slice(2, 9)}`;
  }, []);

  // Clear timer for a specific toast
  const clearTimer = useCallback((id: string) => {
    const timer = timersRef.current.get(id);
    if (timer) {
      clearTimeout(timer);
      timersRef.current.delete(id);
    }
  }, []);

  // Remove toast by ID
  const removeToast = useCallback((id: string) => {
    clearTimer(id);
    setToasts((prev) => prev.filter((t) => t.id !== id));
  }, [clearTimer]);

  // Add new toast
  const addToast = useCallback((toast: Omit<Toast, 'id'>): string => {
    const id = generateId();
    const duration = toast.duration ?? DEFAULT_DURATIONS[toast.type];

    const newToast: Toast = {
      ...toast,
      id,
      duration,
    };

    setToasts((prev) => {
      // Limit max toasts, remove oldest if needed
      const updated = [...prev, newToast];
      if (updated.length > maxToasts) {
        const removed = updated.shift();
        if (removed) {
          clearTimer(removed.id);
        }
      }
      return updated;
    });

    // Set auto-dismiss timer if duration > 0
    if (duration > 0) {
      const timer = setTimeout(() => {
        removeToast(id);
      }, duration);
      timersRef.current.set(id, timer);
    }

    return id;
  }, [generateId, maxToasts, clearTimer, removeToast]);

  // Clear all toasts
  const clearAllToasts = useCallback(() => {
    // Clear all timers
    timersRef.current.forEach((timer) => clearTimeout(timer));
    timersRef.current.clear();
    setToasts([]);
  }, []);

  // Cleanup timers on unmount
  useEffect(() => {
    return () => {
      timersRef.current.forEach((timer) => clearTimeout(timer));
      timersRef.current.clear();
    };
  }, []);

  const contextValue: ToastContextValue = {
    toasts,
    addToast,
    removeToast,
    clearAllToasts,
  };

  return (
    <ToastContext.Provider value={contextValue}>
      {children}
    </ToastContext.Provider>
  );
}

/**
 * Hook to access toast context.
 * Must be used within a ToastProvider.
 */
export function useToast(): ToastContextValue {
  const context = useContext(ToastContext);
  if (context === undefined) {
    throw new Error('useToast must be used within a ToastProvider');
  }
  return context;
}

/**
 * Convenience hook for common toast actions.
 */
export function useToastActions() {
  const { addToast } = useToast();

  const success = useCallback((title: string, message?: string) => {
    return addToast({ type: 'success', title, message });
  }, [addToast]);

  const error = useCallback((title: string, message?: string) => {
    return addToast({ type: 'error', title, message });
  }, [addToast]);

  const warning = useCallback((title: string, message?: string) => {
    return addToast({ type: 'warning', title, message });
  }, [addToast]);

  const info = useCallback((title: string, message?: string) => {
    return addToast({ type: 'info', title, message });
  }, [addToast]);

  return { success, error, warning, info, addToast };
}
