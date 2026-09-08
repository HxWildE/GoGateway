# Episode 10: Systems & Automation Essentials

## 📖 The Story: The Corporate HR Department (Active Directory)
Imagine running a company with 10,000 employees. If an employee gets fired, you don't want to physically walk to 50 different servers to delete their login account.
- **Active Directory (AD)** is the central HR database for IT. It holds all Users, Computers, and Groups. When someone logs in, the PC asks AD, "Is this password correct?" (via the **Kerberos** protocol). If you disable an account in AD, they are instantly locked out of everything on the network.

## 🤿 Layer 1 Deep Dive: Active Directory Core Concepts
AD is structured logically into a hierarchy.
1. **Domain:** The main boundary (e.g., `corp.mycompany.com`). Handled by Domain Controller (DC) servers.
2. **OUs (Organizational Units):** Folders inside the Domain to organize things. (e.g., an OU for "Sales Users", an OU for "Sales Laptops").
3. **GPOs (Group Policy Objects):** The true power of AD. You apply a GPO to an OU to enforce settings globally. Want to disable the USB ports on all Sales Laptops? Force a specific desktop wallpaper? Map a network drive? You write one GPO, link it to the "Sales Laptops" OU, and hundreds of machines obey instantly.

## 🤿 Layer 2 Deep Dive: Windows vs Linux Permissions & Scripting
As a modern engineer, you will work across both OS environments.

**Permissions:**
- **Windows (NTFS):** Uses detailed ACLs (Access Control Lists). You can get hyper-granular (e.g., User A can read/write, User B can only execute, User C is explicitly denied).
- **Linux (`chmod` / `chown`):** Uses a simpler, rigid octal system. Permissions are assigned to the **Owner**, the **Group**, and **Others** (Everyone else). 
  - `Read (4)`, `Write (2)`, `Execute (1)`.
  - `chmod 755 script.sh` means: Owner gets 7 (4+2+1, full control). Group gets 5 (4+1, read/execute). Others get 5. 

**Operations Scripting (Why do we code?):**
We don't script to build web apps; we script to *eliminate toil*. 
- **PowerShell (Windows):** Object-oriented. When you run `Get-Process`, it doesn't return text; it returns an array of Process Objects. You can pipe `|` them into `Stop-Process` effortlessly.
- **Bash (Linux):** Text-oriented. You use `grep`, `awk`, and `sed` to slice and dice string outputs from commands.
- **Python:** The universal glue. Used to hit REST APIs (e.g., pulling ticket data from Jira or configuring a Meraki firewall via API).

### Visualizing GPO Application
```mermaid
graph TD
    A[Domain: corp.local] --> B{OU: HR Department}
    A --> C{OU: IT Department}
    
    B --> D(User: Alice)
    B --> E(PC: HR-Laptop-01)
    
    F[GPO: Disable USB Drives] -.->|Linked to| B
    
    Note over B,F: Alice and HR-Laptop-01 instantly <br>lose USB drive access. IT Department is unaffected.
```

## 🎙️ The Interview Answer (Memorize This)
**Interviewer:** *"Can you explain what Active Directory is and why Group Policy (GPO) is important?"*
**You:** *"Active Directory is a centralized identity and access management service developed by Microsoft. It authenticates and authorizes all users and computers in a Windows domain using Kerberos. Group Policy Objects, or GPOs, are incredibly important because they allow administrators to centrally manage, configure, and secure thousands of endpoints simultaneously—from enforcing password complexity rules to mapping network drives—without ever touching the individual machines."*

## 🕵️ The Interrogation (Read, Answer, Analyze)

**Q1: A user is in the "Finance" Security Group. The Finance folder has NTFS permissions granting the Finance Group 'Read' access. However, the user also has a specific individual NTFS permission explicitly 'Denying' them access. Can the user read the folder?**
- **Analysis/Answer:** No. In Windows NTFS permissions, an **Explicit Deny** always overrides any Allow permissions, regardless of group memberships. The system checks for Deny rules first; if one matches, it immediately blocks access.

**Q2: You write a Bash script `backup.sh` on a Linux server. When you type `./backup.sh`, the system says "Permission Denied". What is the command to fix this and why?**
- **Analysis/Answer:** You must run `chmod +x backup.sh`. By default, newly created files in Linux do not have the Execute bit set for security reasons. You have to explicitly grant execute permissions to turn the text file into a runnable script.

**Q3: Why would a network engineer use Python with the `requests` library instead of just logging into a router's web interface?**
- **Analysis/Answer:** Scalability and automation. Logging into a web interface is fine for one router. But if a vulnerability requires you to update a firewall rule on 500 routers globally by midnight, doing it manually via a GUI is impossible. With Python and the `requests` library, you can write a 20-line script to loop through all 500 IP addresses, hit their REST APIs, apply the rule, and generate a success/failure report in seconds.
