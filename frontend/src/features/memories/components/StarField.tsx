/**
 * StarField Component - Animated Cosmic Background for Graph
 * 
 * Purpose:
 * Creates an animated starfield background effect for the graph visualization.
 * Extracted from GraphVisualization to improve component separation and reusability.
 * 
 * Key Features:
 * - Animated cosmic stars with different types and behaviors
 * - Twinkling effects with varied timing
 * - Parallax movement simulation
 * - Performance-optimized canvas rendering
 * - Responsive to container size changes
 * 
 * Star Types:
 * - Bright stars (10%): Larger, brighter, faster twinkling
 * - Normal stars (60%): Medium size and brightness
 * - Distant stars (30%): Small, dim, slow twinkling
 * 
 * Performance:
 * - Uses requestAnimationFrame for smooth animations
 * - Efficiently manages star lifecycle and rendering
 * - Automatically cleans up resources on unmount
 */

import React, { useEffect, useRef, memo } from 'react';

interface Star {
    x: number;
    y: number;
    size: number;
    opacity: number;
    twinkleSpeed: number;
    type: 'normal' | 'bright' | 'distant';
}

interface StarFieldProps {
    /** Width of the container */
    width?: number;
    /** Height of the container */
    height?: number;
    /** Number of stars to render (default: 200) */
    starCount?: number;
    /** Whether animations are enabled */
    animate?: boolean;
    /** Optional class name for the canvas */
    className?: string;
}

const StarField: React.FC<StarFieldProps> = ({
    width,
    height,
    starCount = 200,
    animate = true,
    className
}) => {
    const canvasRef = useRef<HTMLCanvasElement>(null);
    const animationRef = useRef<number | undefined>(undefined);
    const starsRef = useRef<Star[]>([]);

    // Initialize stars
    const initializeStars = (canvasWidth: number, canvasHeight: number) => {
        const stars: Star[] = [];
        
        for (let i = 0; i < starCount; i++) {
            const starType = Math.random();
            let size: number, opacity: number, twinkleSpeed: number, type: 'normal' | 'bright' | 'distant';
            
            if (starType < 0.1) {
                // Bright stars (10%)
                size = Math.random() * 2 + 1.5;
                opacity = 1;  // Maximum brightness for bright stars
                twinkleSpeed = Math.random() * 2000 + 1000;
                type = 'bright';
            } else if (starType < 0.7) {
                // Normal stars (60%)
                size = Math.random() * 1 + 0.5;
                opacity = Math.random() * 0.2 + 0.8;  // 0.8-1.0 range
                twinkleSpeed = Math.random() * 3000 + 2000;
                type = 'normal';
            } else {
                // Distant stars (30%)
                size = Math.random() * 0.5 + 0.2;
                opacity = Math.random() * 0.3 + 0.6;  // 0.6-0.9 range
                twinkleSpeed = Math.random() * 4000 + 3000;
                type = 'distant';
            }
            
            stars.push({
                x: Math.random() * canvasWidth,
                y: Math.random() * canvasHeight,
                size,
                opacity,
                twinkleSpeed,
                type
            });
        }
        
        starsRef.current = stars;
    };

    // Render stars on canvas
    const renderStars = (ctx: CanvasRenderingContext2D, canvasWidth: number, canvasHeight: number) => {
        ctx.clearRect(0, 0, canvasWidth, canvasHeight);
        
        const time = Date.now();
        
        starsRef.current.forEach((star: Star) => {
            ctx.save(); // Save context state to ensure clean rendering
            
            // Enhanced twinkling effect based on star type
            const twinkle = Math.sin(time / star.twinkleSpeed) * 0.5 + 0.5;
            const currentOpacity = star.opacity * (0.7 + twinkle * 0.3);  // Higher minimum brightness (70%)
            
            // Different colors for different star types - all much brighter
            let color: string;
            switch (star.type) {
                case 'bright':
                    color = `rgba(255, 255, 255, ${currentOpacity})`;
                    // Strong glow for bright stars
                    ctx.shadowColor = 'rgba(255, 255, 255, 1)';
                    ctx.shadowBlur = star.size * 4;
                    break;
                case 'normal':
                    color = `rgba(240, 245, 255, ${currentOpacity})`;  // Much brighter
                    ctx.shadowColor = 'rgba(240, 245, 255, 0.9)';  // Stronger glow
                    ctx.shadowBlur = star.size * 2;
                    break;
                case 'distant':
                    color = `rgba(210, 210, 240, ${currentOpacity})`;  // Much brighter
                    ctx.shadowColor = 'rgba(210, 210, 240, 0.7)';  // Visible glow
                    ctx.shadowBlur = star.size * 1;
                    break;
            }
            
            // Ensure perfect circles by using proper arc rendering
            ctx.beginPath();
            ctx.arc(Math.round(star.x), Math.round(star.y), star.size, 0, Math.PI * 2, false);
            ctx.closePath();
            ctx.fillStyle = color;
            ctx.fill();
            
            ctx.restore(); // Restore context state
            
            if (animate) {
                // Slowly move stars (slower for distant stars)
                const moveSpeed = star.type === 'distant' ? 0.01 : star.type === 'bright' ? 0.035 : 0.02;
                star.y -= moveSpeed;
                
                // Reset stars that go off screen
                if (star.y < -star.size) {
                    star.y = canvasHeight + star.size;
                    star.x = Math.random() * canvasWidth;
                }
            }
        });
    };

    // Animation loop
    const animateStars = () => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        
        const ctx = canvas.getContext('2d');
        if (!ctx) return;
        
        renderStars(ctx, canvas.width, canvas.height);
        
        if (animate) {
            animationRef.current = requestAnimationFrame(animateStars);
        }
    };

    useEffect(() => {
        const canvas = canvasRef.current;
        if (!canvas) return;
        
        const ctx = canvas.getContext('2d');
        if (!ctx) return;
        
        // Set canvas size
        const canvasWidth = width || canvas.offsetWidth;
        const canvasHeight = height || canvas.offsetHeight;
        
        canvas.width = canvasWidth;
        canvas.height = canvasHeight;
        
        // Initialize stars and start animation
        initializeStars(canvasWidth, canvasHeight);
        
        if (animate) {
            animateStars();
        } else {
            renderStars(ctx, canvasWidth, canvasHeight);
        }
        
        return () => {
            if (animationRef.current) {
                cancelAnimationFrame(animationRef.current);
            }
        };
    }, [width, height, starCount, animate]);

    return <canvas ref={canvasRef} className={className} />;
};

export default memo(StarField);
