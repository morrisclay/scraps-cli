# TypeScript Utility Functions Library

Build a collection of independent utility functions. Each function is in its own file with NO dependencies on other files.

## Structure
- Each function in `src/utils/{name}.ts`
- NO dependencies between files (100% parallel)
- Pure TypeScript

## Functions to Implement

1. `slugify.ts` - Convert string to URL-safe slug (lowercase, hyphens)
2. `capitalize.ts` - Capitalize first letter of each word
3. `truncate.ts` - Truncate string with "..." at max length
4. `debounce.ts` - Debounce function calls
5. `throttle.ts` - Throttle function calls
6. `deepClone.ts` - Deep clone an object
7. `flatten.ts` - Flatten nested arrays
8. `unique.ts` - Get unique values from array
9. `groupBy.ts` - Group array items by key function
10. `chunk.ts` - Split array into chunks of size N
11. `shuffle.ts` - Randomly shuffle array
12. `pick.ts` - Pick specific keys from object
13. `omit.ts` - Omit specific keys from object
14. `merge.ts` - Deep merge objects
15. `isEmpty.ts` - Check if value is empty (null, undefined, [], {}, "")
16. `isEqual.ts` - Deep equality check
17. `randomInt.ts` - Generate random integer in range
18. `sleep.ts` - Promise-based delay function
19. `retry.ts` - Retry async function with exponential backoff
20. `memoize.ts` - Memoize function results

## Requirements
- Each function is self-contained
- TypeScript with proper typing
- Export default function from each file
- Include JSDoc comments
