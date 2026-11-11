"use client";

import React, { createContext, useContext, useEffect, useState } from "react";

type Theme = "dark" | "light";

type PreferencesContextValue = {
  theme: Theme;
  toggleTheme: () => void;
  bilingual: boolean;
  toggleBilingual: () => void;
};

const PreferencesContext = createContext<PreferencesContextValue>({
  theme: "dark",
  toggleTheme: () => {},
  bilingual: true,
  toggleBilingual: () => {}
});

export function PreferencesProvider({ children }: { children: React.ReactNode }) {
  const [theme, setTheme] = useState<Theme>("dark");
  const [bilingual, setBilingual] = useState<boolean>(true);

  useEffect(() => {
    try {
      const savedTheme = window.localStorage.getItem("ariba_ui_theme");
      if (savedTheme === "light" || savedTheme === "dark") {
        setTheme(savedTheme);
        document.documentElement.setAttribute("data-theme", savedTheme);
      } else {
        document.documentElement.setAttribute("data-theme", "dark");
      }
      const savedBn = window.localStorage.getItem("ariba_ui_bilingual");
      if (savedBn === "0") {
        setBilingual(false);
      }
    } catch {
      document.documentElement.setAttribute("data-theme", "dark");
    }
  }, []);

  function toggleTheme() {
    setTheme((prev) => {
      const next: Theme = prev === "dark" ? "light" : "dark";
      document.documentElement.setAttribute("data-theme", next);
      try {
        window.localStorage.setItem("ariba_ui_theme", next);
      } catch {
        // ignore storage errors
      }
      return next;
    });
  }

  function toggleBilingual() {
    setBilingual((prev) => {
      const next = !prev;
      try {
        window.localStorage.setItem("ariba_ui_bilingual", next ? "1" : "0");
      } catch {
        // ignore storage errors
      }
      return next;
    });
  }

  return (
    <PreferencesContext.Provider value={{ theme, toggleTheme, bilingual, toggleBilingual }}>
      {children}
    </PreferencesContext.Provider>
  );
}

export function usePreferences() {
  return useContext(PreferencesContext);
}
