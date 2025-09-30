class ConfettiCannon {
    constructor() {
        this.canvas = null;
        this.ctx = null;
        this.particles = [];
        this.animationId = null;
        this.isAnimating = false;
    }

    init() {
        this.canvas = document.getElementById('celebration-canvas');
        if (!this.canvas) {
            this.canvas = document.createElement('canvas');
            this.canvas.id = 'celebration-canvas';
            this.canvas.style.cssText = `
                position: fixed;
                top: 0;
                left: 0;
                width: 100%;
                height: 100%;
                pointer-events: none;
                z-index: 9999;
            `;
            document.body.appendChild(this.canvas);
        }

        this.ctx = this.canvas.getContext('2d');
        this.resize();
        window.addEventListener('resize', () => this.resize());
    }

    resize() {
        this.canvas.width = window.innerWidth;
        this.canvas.height = window.innerHeight;
    }

    createConfetti(x, y, count = 100) {
        const colors = ['#667eea', '#764ba2', '#f093fb', '#4facfe', '#43e97b', '#fa709a', '#fee140', '#30cfd0'];
        const shapes = ['circle', 'square', 'triangle', 'star'];

        for (let i = 0; i < count; i++) {
            this.particles.push({
                x,
                y,
                vx: (Math.random() - 0.5) * 15,
                vy: Math.random() * -15 - 5,
                rotation: Math.random() * 360,
                rotationSpeed: (Math.random() - 0.5) * 10,
                color: colors[Math.floor(Math.random() * colors.length)],
                shape: shapes[Math.floor(Math.random() * shapes.length)],
                size: Math.random() * 8 + 4,
                gravity: 0.3 + Math.random() * 0.2,
                life: 1.0,
                decay: 0.01 + Math.random() * 0.01
            });
        }
    }

    createFirework(x, y) {
        const colors = ['#667eea', '#764ba2', '#f093fb', '#4facfe', '#43e97b', '#fa709a', '#fee140', '#30cfd0'];
        const particleCount = 50;
        const color = colors[Math.floor(Math.random() * colors.length)];

        for (let i = 0; i < particleCount; i++) {
            const angle = (Math.PI * 2 * i) / particleCount;
            const velocity = 5 + Math.random() * 3;

            this.particles.push({
                x,
                y,
                vx: Math.cos(angle) * velocity,
                vy: Math.sin(angle) * velocity,
                rotation: 0,
                rotationSpeed: 0,
                color,
                shape: 'circle',
                size: 3,
                gravity: 0.1,
                life: 1.0,
                decay: 0.02,
                trail: true
            });
        }
    }

    createGlitter(x, y, count = 30) {
        const glitterColors = ['#ffd700', '#ffed4e', '#fff68f', '#fafad2', '#ffffff'];

        for (let i = 0; i < count; i++) {
            this.particles.push({
                x,
                y,
                vx: (Math.random() - 0.5) * 8,
                vy: (Math.random() - 0.5) * 8,
                rotation: Math.random() * 360,
                rotationSpeed: (Math.random() - 0.5) * 20,
                color: glitterColors[Math.floor(Math.random() * glitterColors.length)],
                shape: 'star',
                size: Math.random() * 6 + 2,
                gravity: 0.05,
                life: 1.0,
                decay: 0.015,
                sparkle: true
            });
        }
    }

    launchConfetti(intensity = 'normal') {
        if (!this.canvas) this.init();

        const counts = {
            light: 50,
            normal: 150,
            mega: 300
        };

        const count = counts[intensity] || counts.normal;
        const width = this.canvas.width;
        const height = this.canvas.height;

        this.createConfetti(width * 0.25, height, count / 3);
        this.createConfetti(width * 0.5, height, count / 3);
        this.createConfetti(width * 0.75, height, count / 3);

        if (intensity === 'mega') {
            setTimeout(() => this.createConfetti(width * 0.1, height, 50), 200);
            setTimeout(() => this.createConfetti(width * 0.9, height, 50), 400);
        }

        this.start();
    }

    launchFireworks(count = 5) {
        if (!this.canvas) this.init();

        const width = this.canvas.width;
        const height = this.canvas.height;

        for (let i = 0; i < count; i++) {
            setTimeout(() => {
                const x = width * (0.2 + Math.random() * 0.6);
                const y = height * (0.2 + Math.random() * 0.3);
                this.createFirework(x, y);
            }, i * 300);
        }

        this.start();
    }

    launchGlitterBurst(x = null, y = null) {
        if (!this.canvas) this.init();

        const posX = x !== null ? x : this.canvas.width / 2;
        const posY = y !== null ? y : this.canvas.height / 2;

        this.createGlitter(posX, posY, 50);

        this.start();
    }

    celebrate() {
        if (!this.canvas) this.init();

        this.launchConfetti('normal');

        setTimeout(() => this.launchFireworks(3), 500);
        setTimeout(() => this.launchGlitterBurst(), 1000);

        setTimeout(() => {
            this.clear();
        }, 5000);
    }

    drawCircle(particle) {
        this.ctx.beginPath();
        this.ctx.arc(particle.x, particle.y, particle.size, 0, Math.PI * 2);
        this.ctx.fillStyle = particle.color;
        this.ctx.globalAlpha = particle.life;
        this.ctx.fill();
    }

    drawSquare(particle) {
        this.ctx.save();
        this.ctx.translate(particle.x, particle.y);
        this.ctx.rotate((particle.rotation * Math.PI) / 180);
        this.ctx.fillStyle = particle.color;
        this.ctx.globalAlpha = particle.life;
        this.ctx.fillRect(-particle.size / 2, -particle.size / 2, particle.size, particle.size);
        this.ctx.restore();
    }

    drawTriangle(particle) {
        this.ctx.save();
        this.ctx.translate(particle.x, particle.y);
        this.ctx.rotate((particle.rotation * Math.PI) / 180);
        this.ctx.beginPath();
        this.ctx.moveTo(0, -particle.size);
        this.ctx.lineTo(particle.size, particle.size);
        this.ctx.lineTo(-particle.size, particle.size);
        this.ctx.closePath();
        this.ctx.fillStyle = particle.color;
        this.ctx.globalAlpha = particle.life;
        this.ctx.fill();
        this.ctx.restore();
    }

    drawStar(particle) {
        this.ctx.save();
        this.ctx.translate(particle.x, particle.y);
        this.ctx.rotate((particle.rotation * Math.PI) / 180);

        const spikes = 5;
        const outerRadius = particle.size;
        const innerRadius = particle.size / 2;

        this.ctx.beginPath();
        for (let i = 0; i < spikes * 2; i++) {
            const radius = i % 2 === 0 ? outerRadius : innerRadius;
            const angle = (Math.PI * i) / spikes;
            const x = Math.cos(angle) * radius;
            const y = Math.sin(angle) * radius;

            if (i === 0) {
                this.ctx.moveTo(x, y);
            } else {
                this.ctx.lineTo(x, y);
            }
        }
        this.ctx.closePath();

        if (particle.sparkle) {
            const gradient = this.ctx.createRadialGradient(0, 0, 0, 0, 0, outerRadius);
            gradient.addColorStop(0, particle.color);
            gradient.addColorStop(1, 'transparent');
            this.ctx.fillStyle = gradient;
        } else {
            this.ctx.fillStyle = particle.color;
        }

        this.ctx.globalAlpha = particle.life;
        this.ctx.fill();
        this.ctx.restore();
    }

    update() {
        this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);

        for (let i = this.particles.length - 1; i >= 0; i--) {
            const p = this.particles[i];

            p.vy += p.gravity;
            p.x += p.vx;
            p.y += p.vy;
            p.rotation += p.rotationSpeed;
            p.life -= p.decay;

            if (p.trail) {
                this.ctx.globalAlpha = p.life * 0.3;
                this.ctx.strokeStyle = p.color;
                this.ctx.lineWidth = p.size;
                this.ctx.beginPath();
                this.ctx.moveTo(p.x - p.vx, p.y - p.vy);
                this.ctx.lineTo(p.x, p.y);
                this.ctx.stroke();
            }

            this.ctx.globalAlpha = 1;

            switch (p.shape) {
                case 'circle':
                    this.drawCircle(p);
                    break;
                case 'square':
                    this.drawSquare(p);
                    break;
                case 'triangle':
                    this.drawTriangle(p);
                    break;
                case 'star':
                    this.drawStar(p);
                    break;
            }

            if (p.life <= 0 || p.y > this.canvas.height + 50) {
                this.particles.splice(i, 1);
            }
        }

        if (this.particles.length > 0) {
            this.animationId = requestAnimationFrame(() => this.update());
        } else {
            this.stop();
            if (this.ctx) {
                this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
            }
        }
    }

    start() {
        if (!this.isAnimating) {
            this.isAnimating = true;
            this.update();
        }
    }

    stop() {
        this.isAnimating = false;
        if (this.animationId) {
            cancelAnimationFrame(this.animationId);
            this.animationId = null;
        }
        if (this.ctx && this.canvas) {
            this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
        }
    }

    clear() {
        this.particles = [];
        this.stop();
        if (this.ctx) {
            this.ctx.clearRect(0, 0, this.canvas.width, this.canvas.height);
        }
    }
}

window.confettiCannon = new ConfettiCannon();
