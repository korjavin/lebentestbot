import os
import fitz  # PyMuPDF
import json

# Load the PDF
pdf_path = "/mnt/data/gesamtfragenkatalog-lebenindeutschland.pdf"
doc = fitz.open(pdf_path)

# Container for questions
questions = []
state_section = False
state_counter = 0
nationwide_counter = 0
max_nationwide = 300
max_state = 160

# Image counter
image_counter = 0
image_folder = "/mnt/data/images"
os.makedirs(image_folder, exist_ok=True)

def extract_question_block(text):
    lines = text.splitlines()
    block = {"Question": "", "Answers": [], "Right answer": -1, "Category": "Nationwide"}
    q_lines = []
    a_lines = []
    for line in lines:
        line = line.strip()
        if line.startswith("") or line.startswith("□"):
            a_lines.append(line[1:].strip())
        elif line:
            q_lines.append(line)
    if q_lines:
        block["Question"] = " ".join(q_lines)
    block["Answers"] = a_lines
    return block

# Iterate through the pages
for page_num in range(len(doc)):
    page = doc.load_page(page_num)
    text = page.get_text()
    images = page.get_images(full=True)

    # Handle images separately
    if images:
        for img_index, img in enumerate(images):
            xref = img[0]
            pix = fitz.Pixmap(doc, xref)
            if pix.n > 4:  # convert CMYK
                pix = fitz.Pixmap(fitz.csRGB, pix)
            image_path = os.path.join(image_folder, f"image_{image_counter}.png")
            pix.save(image_path)
            pix = None
            image_counter += 1
            # replace image question block
            questions.append({
                "Question": f"[IMAGE]",
                "Image": image_path,
                "Answers": [],
                "Right answer": -1,
                "Category": "Nationwide" if not state_section else "State-specific"
            })
            continue

    # Parse questions and answers
    if "Fragen zum Bundesland" in text:
        state_section = True
        continue

    blocks = text.split("Aufgabe ")
    for block in blocks:
        if block.strip() == "":
            continue
        if state_section and state_counter < max_state:
            q = extract_question_block(block)
            q["Category"] = "State-specific"
            questions.append(q)
            state_counter += 1
        elif not state_section and nationwide_counter < max_nationwide:
            q = extract_question_block(block)
            q["Category"] = "Nationwide"
            questions.append(q)
            nationwide_counter += 1

# Save to JSON
json_path = "/mnt/data/leben_in_deutschland_questions.json"
with open(json_path, "w", encoding="utf-8") as f:
    json.dump(questions, f, ensure_ascii=False, indent=2)

(json_path, len(questions), nationwide_counter, state_counter, image_counter)
