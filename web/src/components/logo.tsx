import { FC } from "react";

interface LogoProps {
    size?: number;
    className?: string;
}

export const Logo: FC<LogoProps> = (props) => {
    const { size = 32, className } = props;

    // The SVG is square, so width = height = size
    const width = size;
    const height = size;

    const fill0 = "#7ca6d8";
    const fill1 = "#5a86c3";
    const fill2 = "#3e5d94";

    return (
        <svg
            width={width}
            height={height}
            viewBox="0 0 2000 2000"
            xmlns="http://www.w3.org/2000/svg"
            className={className}
        >
            <g>
                <g>
                    <path
                        d="m1664.4 1000.5c-5.2-5.7-10.6-11.3-16.1-16.8l-400.7-400.6-417.5 417.4-582.6-582.6c-135.8-135.8-39.6-367.9 152.4-367.9h847.7c497 0 749.8 591.6 416.8 950.5z"
                        fill={fill0}
                    />
                </g>
                <path
                    d="m1664.4 1000.5c-5.2 5.6-10.6 11.2-16.1 16.7l-400.7 400.8-417.5-417.5 417.5-417.4 400.7 400.6c5.5 5.5 10.9 11.1 16.1 16.8z"
                    fill={fill1}
                />
                <g>
                    <path
                        d="m1247.6 1951h-847.7c-192 0-288.1-232.1-152.3-367.9l582.5-582.6 417.5 417.5 400.7-400.8c5.5-5.5 10.9-11.1 16.1-16.7 332.9 358.9 80.2 950.5-416.8 950.5z"
                        fill={fill2}
                    />
                </g>
            </g>
        </svg>
    );
};
