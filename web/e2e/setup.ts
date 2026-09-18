import { execFileSync } from 'node:child_process';
export default function setup() { execFileSync(process.env.PYTHON || 'python', ['../scripts/ui-fixture.py', 'create'], { stdio: 'inherit' }); }
