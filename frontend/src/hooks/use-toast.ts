import { toast as sonnerToast } from 'sonner';

interface ToastOptions {
  title?: string;
  description?: string;
  variant?: 'default' | 'destructive';
  duration?: number;
}

export function useToast() {
  const toast = (options: ToastOptions) => {
    const { title, description, variant = 'default', duration } = options;

    if (variant === 'destructive') {
      return sonnerToast.error(title, {
        description,
        duration,
      });
    }

    return sonnerToast.success(title, {
      description,
      duration,
    });
  };

  return { toast };
}
