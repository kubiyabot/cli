import { h } from 'preact';
import { useEffect, useRef, useCallback } from 'preact/hooks';

export interface ConfirmDialogProps {
  /** Whether the dialog is open */
  isOpen: boolean;
  /** Dialog title */
  title: string;
  /** Dialog message/description */
  message: string;
  /** Text for confirm button */
  confirmLabel?: string;
  /** Text for cancel button */
  cancelLabel?: string;
  /** Visual variant - affects confirm button styling */
  variant?: 'danger' | 'warning' | 'default';
  /** Called when user confirms */
  onConfirm: () => void;
  /** Called when user cancels (ESC, cancel button, or backdrop click) */
  onCancel: () => void;
  /** Shows loading state on confirm button */
  isLoading?: boolean;
  /** Additional children to render in the dialog body */
  children?: preact.ComponentChildren;
}

// Icons
const WarningIcon = (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z" />
    <line x1="12" y1="9" x2="12" y2="13" />
    <line x1="12" y1="17" x2="12.01" y2="17" />
  </svg>
);

const DangerIcon = (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="10" />
    <line x1="15" y1="9" x2="9" y2="15" />
    <line x1="9" y1="9" x2="15" y2="15" />
  </svg>
);

const InfoIcon = (
  <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="12" cy="12" r="10" />
    <line x1="12" y1="16" x2="12" y2="12" />
    <line x1="12" y1="8" x2="12.01" y2="8" />
  </svg>
);

/**
 * Confirmation dialog component to replace browser confirm() calls.
 * Supports keyboard navigation, focus trap, and loading states.
 */
export function ConfirmDialog({
  isOpen,
  title,
  message,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  variant = 'default',
  onConfirm,
  onCancel,
  isLoading = false,
  children,
}: ConfirmDialogProps) {
  const dialogRef = useRef<HTMLDivElement>(null);
  const confirmBtnRef = useRef<HTMLButtonElement>(null);
  const cancelBtnRef = useRef<HTMLButtonElement>(null);
  const previousActiveElement = useRef<Element | null>(null);

  // Store the previously focused element when opening
  useEffect(() => {
    if (isOpen) {
      previousActiveElement.current = document.activeElement;
      // Focus the cancel button by default (safer option)
      setTimeout(() => {
        cancelBtnRef.current?.focus();
      }, 0);
    } else {
      // Return focus to the previous element
      if (previousActiveElement.current instanceof HTMLElement) {
        previousActiveElement.current.focus();
      }
    }
  }, [isOpen]);

  // Handle keyboard events
  useEffect(() => {
    if (!isOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      // ESC closes the dialog
      if (e.key === 'Escape' && !isLoading) {
        e.preventDefault();
        onCancel();
        return;
      }

      // Enter confirms (only if confirm button is focused or no input is focused)
      if (e.key === 'Enter' && !isLoading) {
        const activeElement = document.activeElement;
        // Only trigger if focused on confirm button
        if (activeElement === confirmBtnRef.current) {
          e.preventDefault();
          onConfirm();
        }
        return;
      }

      // Tab focus trap
      if (e.key === 'Tab') {
        const focusableElements = dialogRef.current?.querySelectorAll(
          'button:not(:disabled), [href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])'
        );

        if (!focusableElements || focusableElements.length === 0) return;

        const firstElement = focusableElements[0] as HTMLElement;
        const lastElement = focusableElements[focusableElements.length - 1] as HTMLElement;

        if (e.shiftKey) {
          // Shift + Tab
          if (document.activeElement === firstElement) {
            e.preventDefault();
            lastElement.focus();
          }
        } else {
          // Tab
          if (document.activeElement === lastElement) {
            e.preventDefault();
            firstElement.focus();
          }
        }
      }
    };

    document.addEventListener('keydown', handleKeyDown);
    return () => document.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, isLoading, onCancel, onConfirm]);

  // Prevent body scroll when open
  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = 'hidden';
    } else {
      document.body.style.overflow = '';
    }
    return () => {
      document.body.style.overflow = '';
    };
  }, [isOpen]);

  // Handle backdrop click
  const handleBackdropClick = useCallback((e: MouseEvent) => {
    if (e.target === e.currentTarget && !isLoading) {
      onCancel();
    }
  }, [isLoading, onCancel]);

  if (!isOpen) return null;

  const icon = variant === 'danger' ? DangerIcon : variant === 'warning' ? WarningIcon : InfoIcon;
  const iconClass = `confirm-dialog-icon confirm-dialog-icon-${variant}`;

  return (
    <div
      class="confirm-dialog-overlay"
      onClick={handleBackdropClick}
      role="presentation"
    >
      <div
        ref={dialogRef}
        class="confirm-dialog"
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-dialog-title"
        aria-describedby="confirm-dialog-message"
      >
        <div class="confirm-dialog-header">
          <div class={iconClass}>
            {icon}
          </div>
          <h2 id="confirm-dialog-title" class="confirm-dialog-title">
            {title}
          </h2>
        </div>

        <div class="confirm-dialog-body">
          <p id="confirm-dialog-message" class="confirm-dialog-message">
            {message}
          </p>
          {children}
        </div>

        <div class="confirm-dialog-actions">
          <button
            ref={cancelBtnRef}
            type="button"
            class="btn btn-secondary"
            onClick={onCancel}
            disabled={isLoading}
          >
            {cancelLabel}
          </button>
          <button
            ref={confirmBtnRef}
            type="button"
            class={`btn ${variant === 'danger' ? 'btn-danger' : variant === 'warning' ? 'btn-warning' : 'btn-primary'}`}
            onClick={onConfirm}
            disabled={isLoading}
          >
            {isLoading && (
              <span class="spinner" style={{ width: '14px', height: '14px' }} />
            )}
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  );
}
