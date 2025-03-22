from common import *

def rsync(src, dst, excludes=[]):
    to_exclude = ['__pycache__']
    to_exclude = to_exclude + excludes
    to_exclude = ' '.join([f'--exclude {e}' for e in to_exclude])
    shell_cmd = f'rsync -e "ssh -o StrictHostKeyChecking=no" -r {to_exclude} {src} {dst}'
    run_shell(shell_cmd)


def main():
    master = SERVERS[0]
    print("copying files to", master)
    path = f'{USERNAME}@{master}:~/'
    print(f'path => {path}')
    rsync(PROJECT_PATH, path, excludes=['.aws-sam', 'benchmark', 'venv', '.git'])

if __name__ == '__main__':
    main()
