import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: "export",
  async rewrites() {
    if (process.env.NODE_ENV === "production") {
      return [];
    }

    return [
      {
        source: "/api/:path*",
        destination: "http://127.0.0.1:9000/api/:path*"
      },
      {
        source: "/health/:path*",
        destination: "http://127.0.0.1:9000/health/:path*"
      }
    ];
  }
};

export default nextConfig;
