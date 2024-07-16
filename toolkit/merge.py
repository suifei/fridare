import os
import argparse

def get_language(file_extension):
    language_map = {
        '.py': 'python',
        '.js': 'javascript',
        '.html': 'html',
        '.css': 'css',
        '.java': 'java',
        '.c': 'c',
        '.cpp': 'cpp',
        '.go': 'golang',
        '.rs': 'rust',
        '.sh': 'shell',
        '.md': 'markdown',
        '.json': 'json',
        '.yaml': 'yaml',
        '.xml': 'xml',
        '.sql': 'sql',
        '.ts': 'typescript',
        
    }
    return language_map.get(file_extension.lower(), '')

def escape_backticks(content):
    return content.replace("```", "\\`\\`\\`")

def merge_files(manifest_file, base_directory, output_file):
    with open(output_file, 'w', encoding='utf-8') as outfile:
        with open(manifest_file, 'r', encoding='utf-8') as manifest:
            for line in manifest:
                file_path = line.strip()
                if not file_path or file_path.startswith('#'):
                    continue  # 跳过空行和注释

                full_path = os.path.join(base_directory, file_path)
                if not os.path.exists(full_path):
                    print(f"Warning: File not found - {full_path}")
                    continue

                _, file_extension = os.path.splitext(file_path)
                
                # 写入文件名作为Markdown标题
                outfile.write(f"# {file_path}\n\n")
                
                # 获取语言类型
                language = get_language(file_extension)
                
                # 读取并转义文件内容
                try:
                    with open(full_path, 'r', encoding='utf-8') as infile:
                        content = infile.read()
                        escaped_content = escape_backticks(content)
                    
                    # 写入文件内容，包括语言类型
                    outfile.write(f"```{language}\n{escaped_content}\n```\n\n")
                except Exception as e:
                    print(f"Error reading file {full_path}: {e}")

def main():
    parser = argparse.ArgumentParser(description="Merge specified files into a single Markdown file.")
    parser.add_argument("manifest", help="Path to the manifest file listing files to merge")
    parser.add_argument("base_directory", help="Base directory for the files listed in the manifest")
    parser.add_argument("output", help="Output file name")
    
    args = parser.parse_args()
    
    merge_files(args.manifest, args.base_directory, args.output)
    print(f"Files merged successfully into {args.output}")

if __name__ == "__main__":
    main()