declare module 'aos' {
    const AOS: {
      init(options?: {
        offset?: number;
        delay?: number;
        duration?: number;
        easing?: string;
        once?: boolean;
        anchorPlacement?: string;
      }): void;
      refresh(): void;
    };
    export default AOS;
  }
  