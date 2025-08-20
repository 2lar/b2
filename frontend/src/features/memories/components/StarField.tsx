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
}

const StarField: React.FC<StarFieldProps> = ({
    width,
    height,
    starCount = 200,
    animate = true
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
                opacity = Math.random() * 0.4 + 0.6;
                twinkleSpeed = Math.random() * 2000 + 1000;
                type = 'bright';
            } else if (starType < 0.7) {
                // Normal stars (60%)
                size = Math.random() * 1 + 0.5;
                opacity = Math.random() * 0.6 + 0.3;
                twinkleSpeed = Math.random() * 3000 + 2000;
                type = 'normal';
            } else {
                // Distant stars (30%)
                size = Math.random() * 0.5 + 0.2;
                opacity = Math.random() * 0.4 + 0.1;
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
            ctx.beginPath();
            
            // Enhanced twinkling effect based on star type
            const twinkle = Math.sin(time / star.twinkleSpeed) * 0.5 + 0.5;
            const currentOpacity = star.opacity * (0.3 + twinkle * 0.7);
            
            // Different colors for different star types
            let color: string;
            switch (star.type) {
                case 'bright':
                    color = `rgba(255, 255, 255, ${currentOpacity})`;
                    // Add subtle glow for bright stars
                    ctx.shadowColor = 'rgba(255, 255, 255, 0.8)';
                    ctx.shadowBlur = star.size * 2;
                    break;
                case 'normal':
                    color = `rgba(200, 220, 255, ${currentOpacity})`;
                    ctx.shadowColor = 'rgba(200, 220, 255, 0.3)';
                    ctx.shadowBlur = star.size;
                    break;
                case 'distant':
                    color = `rgba(150, 150, 200, ${currentOpacity})`;
                    ctx.shadowColor = 'transparent';
                    ctx.shadowBlur = 0;
                    break;
            }
            
            ctx.arc(star.x, star.y, star.size, 0, Math.PI * 2);
            ctx.fillStyle = color;
            ctx.fill();
            
            // Reset shadow for next star
            ctx.shadowColor = 'transparent';
            ctx.shadowBlur = 0;
            
            if (animate) {
                // Slowly move stars (slower for distant stars)
                const moveSpeed = star.type === 'distant' ? 0.02 : star.type === 'bright' ? 0.08 : 0.05;
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

    return (
        <canvas
            ref={canvasRef}
            className="star-background"
            style={{
                position: 'absolute',
                top: 0,
                left: 0,
                width: '100%',
                height: '100%',
                pointerEvents: 'none',
                zIndex: 0,
            }}
        />
    );
};

export default memo(StarField);