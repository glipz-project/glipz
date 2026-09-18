import { execFileSync } from 'node:child_process';
export default function cleanup() { execFileSync(process.env.PYTHON || 'python', ['../scripts/ui-fixture.py', 'cleanup'], { stdio: 'inherit' }); }
