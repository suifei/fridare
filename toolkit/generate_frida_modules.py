import os
import re
import requests

def get_proxy_settings():
    curl_proxy = os.environ.get('CURL_PROXY', '')
    if curl_proxy:
        return {"http": curl_proxy, "https": curl_proxy}
    return None

def get_latest_release():
    url = "https://api.github.com/repos/frida/frida/releases/latest"
    proxies = get_proxy_settings()
    try:
        response = requests.get(url, proxies=proxies)
        response.raise_for_status()
        return response.json()
    except requests.RequestException as e:
        print(f"Error fetching latest release: {str(e)}")
        return None

def parse_filename(filename):
    pattern = re.compile(r'(?P<module_type>frida[-_]?[a-zA-Z0-9\-]+)?[-_](?P<version>v?\d+\.\d+\.\d+)[-_](?P<os>[a-zA-Z0-9\-]+)[-_](?P<arch>[a-zA-Z0-9_]+)')
    match = pattern.search(filename)
    if match:
        module_type = match.group('module_type') or 'N/A'
        version = match.group('version')
        os = match.group('os')
        arch = match.group('arch')
        if '.' in filename:
            ext = filename.split('.')[-1]
        else:
            ext = 'N/A'
        if '64' == arch:
            arch = 'x86_64'
        elif '_' in arch:
            arch = arch.split('_')[-1]
        if os.startswith('cp') and '-' in os:
            os_info = os.split('-')
            os = os_info[-1]
            module_type = 'frida-python-' + os_info[0] + '-' + os_info[1]
        elif 'node-' in os:
            os_info = os.split('-')
            os = os_info[-1]
            module_type = 'frida-' + os_info[0] + '-' + os_info[1]
        elif 'electron-' in os:
            os_info = os.split('-')
            os = os_info[-1]
            module_type = 'frida-' + os_info[0] + '-' + os_info[1]
        elif '-' in os:
            os = os.split('-')[0]

        if 'N/A' == module_type and 'deb' in filename:
            module_type = 'frida-' + os + '-deb'
        elif 'N/A' == module_type and 'gum-graft' in filename:
            module_type = 'gum-graft'
            os = filename.split('-')[-2]
            arch = filename.split('-')[-1].split('.')[0]
        return (module_type, version, os, arch, ext)
    return None

def generate_frida_modules():
    release = get_latest_release()
    if not release:
        return []

    assets = release['assets']
    frida_modules = []

    for asset in assets:
        parsed = parse_filename(asset['name'])
        if parsed:
            module_type, version, os, arch, ext = parsed
            if 'v' == version[0]:
                version = version[1:]
            asset_name = asset['name'].replace(version, '{VERSION}')
            frida_modules.append(f'"{module_type}:{os}:{arch}:{asset_name}"')

    return frida_modules

def main():
    frida_modules = generate_frida_modules()
    print("FRIDA_MODULES=(")
    for module in frida_modules:
        print(f"    {module}")
    print(")")

if __name__ == "__main__":
    main()