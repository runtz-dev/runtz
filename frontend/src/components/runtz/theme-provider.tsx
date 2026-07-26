"use client"

import * as React from "react"
import { MoonIcon, SunIcon } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "@/components/ui/tooltip"

type Theme = "white" | "black"

type ThemeContextValue = {
  theme: Theme
  setTheme: (theme: Theme) => void
  toggleTheme: () => void
}

const THEME_STORAGE_KEY = "runtz_theme"
const ThemeContext = React.createContext<ThemeContextValue | null>(null)

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setThemeState] = React.useState<Theme>("black")

  React.useEffect(() => {
    const storedTheme = window.localStorage.getItem(THEME_STORAGE_KEY)
    setThemeState(storedTheme === "white" ? "white" : "black")
  }, [])

  React.useEffect(() => {
    document.documentElement.classList.toggle("dark", theme === "black")
    document.documentElement.style.colorScheme =
      theme === "black" ? "dark" : "light"
    window.localStorage.setItem(THEME_STORAGE_KEY, theme)
  }, [theme])

  const value = React.useMemo<ThemeContextValue>(
    () => ({
      theme,
      setTheme: setThemeState,
      toggleTheme: () =>
        setThemeState((current) => (current === "black" ? "white" : "black")),
    }),
    [theme]
  )

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>
}

export function useTheme() {
  const context = React.useContext(ThemeContext)
  if (!context) {
    throw new Error("useTheme must be used inside ThemeProvider")
  }

  return context
}

export function ThemeToggle() {
  const { theme, toggleTheme } = useTheme()
  const nextTheme = theme === "black" ? "white" : "black"
  const label = nextTheme === "black" ? "Black theme" : "White theme"

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              aria-label={label}
              className="rounded-full border-[#2f7eff]/20 bg-[#dcecff]/70 text-[#1d5fc7] hover:border-[#2f7eff]/45 hover:bg-[#d8ebff] hover:text-[#071222] dark:border-[#6db5ff]/24 dark:bg-[#101827]/80 dark:text-[#d9e9ff] dark:hover:border-[#6db5ff]/45 dark:hover:bg-[#172844] dark:hover:text-[#eaf4ff]"
              size="icon"
              type="button"
              variant="outline"
              onClick={toggleTheme}
            >
              {theme === "black" ? <SunIcon /> : <MoonIcon />}
            </Button>
          }
        />
        <TooltipContent>{label}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
