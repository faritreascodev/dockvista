import { useMemo, useState } from "react";
import { SearchContext } from "./useSearch";

/** One search box in the top bar, shared across pages — each list page
 * filters its own rows by whatever field makes sense for it (name, image,
 * tag, ...). Cleared automatically isn't needed since each page filters
 * independently; a query that matches nothing on the current page just
 * shows its own empty state. */
export function SearchProvider({ children }: { children: React.ReactNode }) {
  const [query, setQuery] = useState("");
  const value = useMemo(() => ({ query, setQuery }), [query]);
  return <SearchContext.Provider value={value}>{children}</SearchContext.Provider>;
}
