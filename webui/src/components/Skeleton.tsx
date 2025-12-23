import { h } from 'preact';

export interface SkeletonProps {
  /** Shape variant of the skeleton */
  variant?: 'text' | 'circular' | 'rectangular';
  /** Width - accepts string ('100%', '200px') or number (converted to px) */
  width?: string | number;
  /** Height - accepts string ('100%', '200px') or number (converted to px) */
  height?: string | number;
  /** Animation type */
  animation?: 'pulse' | 'wave' | 'none';
  /** Additional CSS class */
  className?: string;
}

/**
 * Converts a size value to a CSS string
 */
function formatSize(size: string | number | undefined): string | undefined {
  if (size === undefined) return undefined;
  if (typeof size === 'number') return `${size}px`;
  return size;
}

/**
 * Skeleton loader component for indicating loading states.
 * Supports text, circular, and rectangular variants with pulse or wave animations.
 */
export function Skeleton({
  variant = 'text',
  width,
  height,
  animation = 'pulse',
  className = '',
}: SkeletonProps) {
  // Default dimensions based on variant
  const defaultWidth = variant === 'circular' ? '40px' : '100%';
  const defaultHeight = variant === 'text' ? '1em' : variant === 'circular' ? '40px' : '100px';

  const style: Record<string, string> = {
    width: formatSize(width) || defaultWidth,
    height: formatSize(height) || defaultHeight,
  };

  // Circular variant gets border-radius: 50%
  if (variant === 'circular') {
    style.borderRadius = '50%';
  }

  const classes = [
    'skeleton',
    `skeleton-${variant}`,
    animation !== 'none' && `skeleton-${animation}`,
    className,
  ].filter(Boolean).join(' ');

  return (
    <div
      class={classes}
      style={style}
      aria-hidden="true"
      role="presentation"
    />
  );
}

/**
 * Preset skeleton for text lines
 */
export function SkeletonText({
  lines = 1,
  width,
  className = '',
}: {
  lines?: number;
  width?: string | number;
  className?: string;
}) {
  return (
    <div class={`skeleton-text-group ${className}`}>
      {Array.from({ length: lines }).map((_, i) => (
        <Skeleton
          key={i}
          variant="text"
          width={i === lines - 1 && lines > 1 ? '80%' : width}
          height="0.875em"
        />
      ))}
    </div>
  );
}

/**
 * Preset skeleton for avatar/profile images
 */
export function SkeletonAvatar({
  size = 40,
  className = '',
}: {
  size?: number;
  className?: string;
}) {
  return (
    <Skeleton
      variant="circular"
      width={size}
      height={size}
      className={className}
    />
  );
}

/**
 * Preset skeleton for cards
 */
export function SkeletonCard({
  className = '',
}: {
  className?: string;
}) {
  return (
    <div class={`skeleton-card ${className}`}>
      <Skeleton variant="rectangular" height={120} />
      <div class="skeleton-card-content">
        <Skeleton variant="text" width="60%" height="1.25em" />
        <Skeleton variant="text" width="90%" />
        <Skeleton variant="text" width="75%" />
      </div>
    </div>
  );
}

/**
 * Preset skeleton for table rows
 */
export function SkeletonTableRow({
  columns = 4,
  className = '',
}: {
  columns?: number;
  className?: string;
}) {
  return (
    <tr class={`skeleton-table-row ${className}`}>
      {Array.from({ length: columns }).map((_, i) => (
        <td key={i}>
          <Skeleton variant="text" width={i === 0 ? '80%' : '60%'} />
        </td>
      ))}
    </tr>
  );
}
