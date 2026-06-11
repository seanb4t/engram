import { clsx, type ClassValue } from 'clsx';
import { twMerge } from 'tailwind-merge';

// cn merges conditional class names (clsx) and de-duplicates conflicting
// Tailwind utilities (tailwind-merge). Standard shadcn-svelte convention.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}
