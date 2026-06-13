declare global {
  interface Window {
    openai?: {
      toolOutput?: any;
      toolResponseMetadata?: any;
      setWidgetState?: (state: any) => void;
      widgetState?: any;
      theme?: 'light' | 'dark';
    };
  }
}

export {};
