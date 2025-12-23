import { h } from 'preact';
import { useToast, type Toast as ToastType } from '../state';

// Icons for each toast type
const ToastIcons = {
  success: (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <polyline points="20 6 9 17 4 12" />
    </svg>
  ),
  error: (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <line x1="18" y1="6" x2="6" y2="18" />
      <line x1="6" y1="6" x2="18" y2="18" />
    </svg>
  ),
  warning: (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
      <line x1="12" y1="9" x2="12" y2="13" />
      <line x1="12" y1="17" x2="12.01" y2="17" />
    </svg>
  ),
  info: (
    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round">
      <circle cx="12" cy="12" r="10" />
      <line x1="12" y1="16" x2="12" y2="12" />
      <line x1="12" y1="8" x2="12.01" y2="8" />
    </svg>
  ),
};

// Close icon
const CloseIcon = (
  <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <line x1="18" y1="6" x2="6" y2="18" />
    <line x1="6" y1="6" x2="18" y2="18" />
  </svg>
);

interface ToastItemProps {
  toast: ToastType;
  onClose: (id: string) => void;
}

/**
 * Individual Toast Item component
 */
function ToastItem({ toast, onClose }: ToastItemProps) {
  const handleActionClick = () => {
    if (toast.action?.onClick) {
      toast.action.onClick();
    }
    onClose(toast.id);
  };

  return (
    <div
      class={`toast toast-${toast.type}`}
      role="alert"
      aria-live={toast.type === 'error' ? 'assertive' : 'polite'}
    >
      <div class="toast-icon">
        {ToastIcons[toast.type]}
      </div>
      <div class="toast-content">
        <div class="toast-title">{toast.title}</div>
        {toast.message && (
          <div class="toast-message">{toast.message}</div>
        )}
        {toast.action && (
          <div class="toast-action">
            <button
              class="toast-action-btn"
              onClick={handleActionClick}
            >
              {toast.action.label}
            </button>
          </div>
        )}
      </div>
      <button
        class="toast-close"
        onClick={() => onClose(toast.id)}
        aria-label="Dismiss notification"
      >
        {CloseIcon}
      </button>
    </div>
  );
}

/**
 * Toast Container component
 * Renders all active toasts in a fixed container
 */
export function ToastContainer() {
  const { toasts, removeToast } = useToast();

  if (toasts.length === 0) {
    return null;
  }

  return (
    <div class="toast-container" aria-label="Notifications">
      {toasts.map((toast) => (
        <ToastItem
          key={toast.id}
          toast={toast}
          onClose={removeToast}
        />
      ))}
    </div>
  );
}
